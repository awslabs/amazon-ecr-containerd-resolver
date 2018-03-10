/*
 * Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var unimplemented = errors.New("unimplemented")

type ecrResolver struct {
	session     *session.Session
	clients     map[string]ecriface.ECRAPI
	clientsLock sync.Mutex
}

func NewResolver(session *session.Session) remotes.Resolver {
	return &ecrResolver{session: session, clients: map[string]ecriface.ECRAPI{}}
}

func (r *ecrResolver) Resolve(ctx context.Context, ref string) (string, ocispec.Descriptor, error) {
	ecrSpec, err := ParseRef(ref)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	if ecrSpec.Object == "" {
		return "", ocispec.Descriptor{}, reference.ErrObjectRequired
	}

	batchGetImageInput := &ecr.BatchGetImageInput{
		RegistryId:         aws.String(ecrSpec.Registry()),
		RepositoryName:     aws.String(ecrSpec.Repository),
		ImageIds:           []*ecr.ImageIdentifier{ecrSpec.ImageID()},
		AcceptedMediaTypes: []*string{aws.String(images.MediaTypeDockerSchema2Manifest)},
	}

	client := r.getClient(ecrSpec.Region())

	batchGetImageOutput, err := client.BatchGetImage(batchGetImageInput)
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
	if len(batchGetImageOutput.Images) != 1 {
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

func (r *ecrResolver) getClient(region string) ecriface.ECRAPI {
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
		ecrBase{
			client:  r.getClient(ecrSpec.Region()),
			ecrSpec: ecrSpec,
		},
	}, nil
}
