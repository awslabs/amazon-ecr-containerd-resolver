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
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	errImageNotFound     = errors.New("ecr: image not found")
	errGetImageUnhandled = errors.New("ecr: unable to get images")

	// supportedImageMediaTypes lists supported content types for images.
	supportedImageMediaTypes = []string{
		ocispec.MediaTypeImageIndex,
		ocispec.MediaTypeImageManifest,
		images.MediaTypeDockerSchema2Manifest,
		images.MediaTypeDockerSchema2ManifestList,
		images.MediaTypeDockerSchema1Manifest,
	}
)

type ecrBase struct {
	client  ecrAPI
	ecrSpec ECRSpec
}

// ecrAPI contains only the ECR APIs that are called by the resolver
// See https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/ecriface/ for the
// full interface from the SDK.
type ecrAPI interface {
	BatchGetImage(context.Context, *ecr.BatchGetImageInput, ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error)
	GetDownloadUrlForLayer(context.Context, *ecr.GetDownloadUrlForLayerInput, ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error)
	BatchCheckLayerAvailability(context.Context, *ecr.BatchCheckLayerAvailabilityInput, ...func(*ecr.Options)) (*ecr.BatchCheckLayerAvailabilityOutput, error)
	InitiateLayerUpload(context.Context, *ecr.InitiateLayerUploadInput, ...func(*ecr.Options)) (*ecr.InitiateLayerUploadOutput, error)
	UploadLayerPart(context.Context, *ecr.UploadLayerPartInput, ...func(*ecr.Options)) (*ecr.UploadLayerPartOutput, error)
	CompleteLayerUpload(context.Context, *ecr.CompleteLayerUploadInput, ...func(*ecr.Options)) (*ecr.CompleteLayerUploadOutput, error)
	PutImage(context.Context, *ecr.PutImageInput, ...func(*ecr.Options)) (*ecr.PutImageOutput, error)
}

// getImage fetches the reference's image from ECR.
func (b *ecrBase) getImage(ctx context.Context) (*types.Image, error) {
	return b.runGetImage(ctx, ecr.BatchGetImageInput{
		ImageIds:           []types.ImageIdentifier{*b.ecrSpec.ImageID()},
		AcceptedMediaTypes: supportedImageMediaTypes,
	})
}

// getImageByDescriptor retrieves an image from ECR for a given OCI descriptor.
func (b *ecrBase) getImageByDescriptor(ctx context.Context, desc ocispec.Descriptor) (*types.Image, error) {
	// If the reference includes both a digest & tag for an image and that
	// digest matches the descriptor's digest then both are specified when
	// requesting an image from ECR. Mutation of the image that pushes an image
	// with a new digest to the tag, will cause the query to fail as the
	// combination of tag AND digest does not match this modified tag.
	//
	// This stronger matching works well for repositories using immutable tags;
	// in the case of immutable tags, a ref like
	// ecr.aws/arn:aws:ecr:us-west-2:111111111111:repository/example-name:tag-name@sha256:$digest
	// would necessarily refer to the same image unless tag-name is deleted and
	// recreated with an different image.
	//
	// Consumers wanting to use a strong reference without assuming immutable
	// tags should instead provide refs that specify digests, excluding its
	// corresponding tag.
	//
	// See the ECR docs on image tag mutability for details:
	//
	// https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-tag-mutability.html
	//
	ident := types.ImageIdentifier{ImageDigest: aws.String(desc.Digest.String())}
	if b.ecrSpec.Spec().Digest() == desc.Digest {
		if tag, _ := b.ecrSpec.TagDigest(); tag != "" {
			ident.ImageTag = aws.String(tag)
		}
	}

	input := ecr.BatchGetImageInput{
		ImageIds: []types.ImageIdentifier{ident},
	}

	// Request exact mediaType when known.
	if desc.MediaType != "" {
		input.AcceptedMediaTypes = []string{desc.MediaType}
	} else {
		input.AcceptedMediaTypes = supportedImageMediaTypes
	}

	return b.runGetImage(ctx, input)
}

// runGetImage submits and handles the response for requests of ECR images.
func (b *ecrBase) runGetImage(ctx context.Context, batchGetImageInput ecr.BatchGetImageInput) (*types.Image, error) {
	// Allow only a single image to be fetched at a time.
	if len(batchGetImageInput.ImageIds) != 1 {
		return nil, errGetImageUnhandled
	}

	batchGetImageInput.RegistryId = aws.String(b.ecrSpec.Registry())
	batchGetImageInput.RepositoryName = aws.String(b.ecrSpec.Repository)

	log.G(ctx).WithField("batchGetImageInput", batchGetImageInput).Trace("ecr.base.image: requesting images")

	batchGetImageOutput, err := b.client.BatchGetImage(ctx, &batchGetImageInput)
	if err != nil {
		log.G(ctx).WithError(err).Error("ecr.base.image: failed to get image")
		return nil, err
	}
	log.G(ctx).WithField("batchGetImageOutput", batchGetImageOutput).Trace("ecr.base.image: api response")

	// Summarize image request failures for handled errors. Only the first
	// failure is checked as only a single ImageIdentifier is allowed to be
	// queried for.
	if len(batchGetImageOutput.Failures) > 0 {
		failure := batchGetImageOutput.Failures[0]
		switch failure.FailureCode {
		// Requested image with a corresponding tag and digest does not exist.
		// This failure will generally occur when pushing an updated (or new)
		// image with a tag.
		case types.ImageFailureCodeImageTagDoesNotMatchDigest:
			log.G(ctx).WithField("failure", failure).Debug("ecr.base.image: no matching image with specified digest")
			return nil, errImageNotFound
		// Requested image doesn't resolve to a known image. A new image will
		// result in an ImageNotFound error when checked before push.
		case types.ImageFailureCodeImageNotFound:
			log.G(ctx).WithField("failure", failure).Debug("ecr.base.image: no image found")
			return nil, errImageNotFound
		// Requested image identifiers are invalid.
		case types.ImageFailureCodeInvalidImageDigest, types.ImageFailureCodeInvalidImageTag:
			log.G(ctx).WithField("failure", failure).Error("ecr.base.image: invalid image identifier")
			return nil, reference.ErrInvalid
		// Unhandled failure reported for image request made.
		default:
			log.G(ctx).WithField("failure", failure).Warn("ecr.base.image: unhandled image request failure")
			return nil, errGetImageUnhandled
		}
	}

	return &batchGetImageOutput.Images[0], nil
}
