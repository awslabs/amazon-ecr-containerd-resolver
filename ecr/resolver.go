/*
 * Copyright 2017-2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You
 * may not use this file except in compliance with the License. A copy of
 * the License is located at
 *
 * 	http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is
 * distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
 * ANY KIND, either express or implied. See the License for the specific
 * language governing permissions and limitations under the License.
 */

package ecr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	ecrsdk "github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var unimplemented = errors.New("unimplemented")

type ecrResolver struct {
	session     *session.Session
	clients     map[string]ecrAPI
	clientsLock sync.Mutex
	tracker     docker.StatusTracker
}

// ResolverOption represents a functional option for configuring the ECR
// Resolver
type ResolverOption func(*ResolverOptions) error

// ResolverOptions represents available options for configuring the ECR Resolver
type ResolverOptions struct {
	// Session is used for configuring the ECR client.  If not specified, a
	// generic session is used.
	Session *session.Session
	// Tracker is used to track uploads to ECR.  If not specified, an in-memory
	// tracker is used instead.
	Tracker docker.StatusTracker
}

// WithSession is a ResolverOption to use a specific AWS session.Session
func WithSession(session *session.Session) ResolverOption {
	return func(options *ResolverOptions) error {
		options.Session = session
		return nil
	}
}

// WithTracker is a ResolverOption to use a specific docker.Tracker
func WithTracker(tracker docker.StatusTracker) ResolverOption {
	return func(options *ResolverOptions) error {
		options.Tracker = tracker
		return nil
	}
}

// NewResolver creates a new remotes.Resolver capable of interacting with Amazon
// ECR.  NewResolver can be called with no arguments for default configuration,
// or can be customized by specifying ResolverOptions.  By default, NewResolver
// will allocate a new AWS session.Session and an in-memory tracker for layer
// progress.
func NewResolver(options ...ResolverOption) (remotes.Resolver, error) {
	resolverOptions := &ResolverOptions{}
	for _, option := range options {
		err := option(resolverOptions)
		if err != nil {
			return nil, err
		}
	}
	if resolverOptions.Session == nil {
		awsSession, err := session.NewSession()
		if err != nil {
			return nil, err
		}
		resolverOptions.Session = awsSession
	}
	if resolverOptions.Tracker == nil {
		resolverOptions.Tracker = docker.NewInMemoryTracker()
	}
	return &ecrResolver{
		session: resolverOptions.Session,
		clients: map[string]ecrAPI{},
		tracker: resolverOptions.Tracker,
	}, nil
}

// Resolve attempts to resolve the provided reference into a name and a
// descriptor.
//
// Valid references are of the form "ecr.aws/arn:aws:ecr:<region>:<account>:repository/<name>:<tag>".
func (r *ecrResolver) Resolve(ctx context.Context, ref string) (string, ocispec.Descriptor, error) {
	ecrSpec, err := ParseRef(ref)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	if ecrSpec.Object == "" {
		return "", ocispec.Descriptor{}, reference.ErrObjectRequired
	}

	batchGetImageInput := &ecr.BatchGetImageInput{
		RegistryId:     aws.String(ecrSpec.Registry()),
		RepositoryName: aws.String(ecrSpec.Repository),
		ImageIds:       []*ecr.ImageIdentifier{ecrSpec.ImageID()},
		AcceptedMediaTypes: []*string{
			aws.String(ocispec.MediaTypeImageManifest),
			aws.String(images.MediaTypeDockerSchema2Manifest),
		},
	}

	client := r.getClient(ecrSpec.Region())

	batchGetImageOutput, err := client.BatchGetImageWithContext(ctx, batchGetImageInput)
	if err != nil {
		log.G(ctx).
			WithField("ref", ref).
			WithError(err).
			Warn("Failed while calling BatchGetImage")
		return "", ocispec.Descriptor{}, err
	}
	log.G(ctx).
		WithField("ref", ref).
		WithField("batchGetImageOutput", batchGetImageOutput).
		Debug("ecr.resolver.resolve")

	var ecrImage *ecr.Image
	if len(batchGetImageOutput.Images) == 0 {
		return "", ocispec.Descriptor{}, reference.ErrInvalid
	}
	ecrImage = batchGetImageOutput.Images[0]
	mediaType := parseImageManifestMediaType(ctx, aws.StringValue(ecrImage.ImageManifest))
	log.G(ctx).
		WithField("ref", ref).
		WithField("media type", mediaType).
		Debug("ecr.resolver.resolve")
	desc := ocispec.Descriptor{
		Digest:    digest.Digest(aws.StringValue(ecrImage.ImageId.ImageDigest)),
		MediaType: mediaType,
		Size:      int64(len(aws.StringValue(ecrImage.ImageManifest))),
	}

	return ecrSpec.Canonical(), desc, nil
}

func (r *ecrResolver) getClient(region string) ecrAPI {
	r.clientsLock.Lock()
	defer r.clientsLock.Unlock()
	if _, ok := r.clients[region]; !ok {
		r.clients[region] = ecrsdk.New(r.session, &aws.Config{Region: aws.String(region)})
	}
	return r.clients[region]
}

type manifestContent struct {
	SchemaVersion int64         `json:"schemaVersion"`
	Signatures    []interface{} `json:"signatures,omitempty"`
	MediaType     string        `json:"mediaType,omitempty"`
}

func parseImageManifestMediaType(ctx context.Context, body string) string {
	var manifest manifestContent
	err := json.Unmarshal([]byte(body), &manifest)
	if err != nil {
		log.G(ctx).WithError(err).Warn("ecr.resolver.resolve: could not parse manifest")
		// default to schema 2 for now
		return images.MediaTypeDockerSchema2Manifest
	}
	if manifest.SchemaVersion == 2 {
		return manifest.MediaType
	} else if manifest.SchemaVersion == 1 {
		if len(manifest.Signatures) == 0 {
			// unsigned
			return "application/vnd.docker.distribution.manifest.v1+json"
		} else {
			return images.MediaTypeDockerSchema1Manifest
		}
	}

	return ""
}

func (r *ecrResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	log.G(ctx).WithField("ref", ref).Debug("ecr.resolver.fetcher")
	ecrSpec, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	return &ecrFetcher{
		ecrBase{
			client:  r.getClient(ecrSpec.Region()),
			ecrSpec: ecrSpec,
		},
	}, nil
}

func (r *ecrResolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	log.G(ctx).WithField("ref", ref).Debug("ecr.resolver.pusher")
	ecrSpec, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	// TODO block pushing by digest since that's not allowed
	// see containerd/remotes/docker/resolver.go:218

	if ecrSpec.Object != "" && strings.Contains(ecrSpec.Object, "@") {
		return nil, errors.New("pusher: cannot use digest reference for push location")
	}

	return &ecrPusher{
		ecrBase: ecrBase{
			client:  r.getClient(ecrSpec.Region()),
			ecrSpec: ecrSpec,
		},
		tracker: r.tracker,
	}, nil
}
