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
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	ecrsdk "github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/containerd/containerd/images"
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
		RegistryId:         aws.String(ecrSpec.Registry),
		RepositoryName:     aws.String(ecrSpec.Repository),
		ImageIds:           []*ecr.ImageIdentifier{ecrSpec.ImageID()},
		AcceptedMediaTypes: []*string{aws.String(images.MediaTypeDockerSchema2Manifest)},
	}

	client := r.getClient(ecrSpec.Region)

	batchGetImageOutput, err := client.BatchGetImage(batchGetImageInput)
	if err != nil {
		fmt.Println(err)
		return "", ocispec.Descriptor{}, err
	}
	fmt.Println(batchGetImageOutput)

	var ecrImage *ecr.Image
	if len(batchGetImageOutput.Images) != 1 {
		return "", ocispec.Descriptor{}, reference.ErrInvalid
	}
	ecrImage = batchGetImageOutput.Images[0]
	desc := ocispec.Descriptor{
		Digest:    digest.Digest(aws.StringValue(ecrImage.ImageId.ImageDigest)),
		MediaType: images.MediaTypeDockerSchema2Manifest, //TODO
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

func (r *ecrResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	fmt.Printf("fetcher: %s\n", ref)
	ecrSpec, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}
	return &ecrFetcher{
		client:  r.getClient(ecrSpec.Region),
		ecrSpec: ecrSpec,
	}, nil
}

func (r *ecrResolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	fmt.Printf("pusher: %s\n", ref)
	return nil, unimplemented
}
