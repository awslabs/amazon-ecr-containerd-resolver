/*
 * Copyright 2017-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	errLayerNotFound = errors.New("ecr: layer not found")
)

// ecrPusher implements the containerd remotes.Pusher interface and can be used
// to push images to Amazon ECR.
type ecrPusher struct {
	ecrBase
}

var _ remotes.Pusher = (*ecrPusher)(nil)

func (p ecrPusher) Push(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("push: desc=%v\n", desc)

	switch desc.MediaType {
	case
		ocispec.MediaTypeImageManifest,
		images.MediaTypeDockerSchema2Manifest,
		images.MediaTypeDockerSchema1Manifest:
		return p.pushManifest(ctx, desc)
	default:
		return p.pushBlob(ctx, desc)
	}

	return nil, unimplemented
}

func (p ecrPusher) pushManifest(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("pushManifest: desc=%v\n", desc)
	exists, err := p.checkManifestExistence(ctx, desc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if exists {
		fmt.Println("exists")
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "content %v on remote", desc.Digest)
	}

	// TODO manifest push
	return nil, errors.New("pushManifest: not implemented")
}

func (p ecrPusher) checkManifestExistence(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	image, err := p.getManifest(ctx)
	if err != nil {
		if err == errImageNotFound {
			return false, nil
		}
		return false, err
	}
	if image == nil {
		return false, errors.New("checkManifestExistence: unexpected nil image")
	}

	found := desc.Digest.String() == aws.StringValue(image.ImageId.ImageDigest)
	return found, nil
}

func (p ecrPusher) pushBlob(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("pushBlob: desc=%v\n", desc)
	exists, err := p.checkBlobExistence(ctx, desc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if exists {
		fmt.Println("exists")
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "content %v on remote", desc.Digest)
	}

	// TODO blob push
	return nil, errors.New("pushBlob: not implemented")
}

func (p ecrPusher) checkBlobExistence(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	fmt.Printf("checkBlobExistence: desc=%v\n", desc)

	batchCheckLayerAvailabilityInput := &ecr.BatchCheckLayerAvailabilityInput{
		RegistryId: aws.String(p.ecrSpec.Registry()),
		RepositoryName: aws.String(p.ecrSpec.Repository),
		LayerDigests: []*string{aws.String(desc.Digest.String())},
	}

	batchCheckLayerAvailabilityOutput, err := p.client.BatchCheckLayerAvailability(batchCheckLayerAvailabilityInput)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	fmt.Println(batchCheckLayerAvailabilityOutput)

	if len(batchCheckLayerAvailabilityOutput.Layers) != 1 {
		if len(batchCheckLayerAvailabilityOutput.Failures) > 0 {
			return false, errLayerNotFound
		}
		return false, reference.ErrInvalid
	}

	return true, nil
}