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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	errImageNotFound = errors.New("ecr: image not found")
)

type ecrBase struct {
	client  ecrAPI
	ecrSpec ECRSpec
}

// ecrAPI contains only the ECR APIs that are called by the resolver
// See https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/ecriface/ for the
// full interface from the SDK.
type ecrAPI interface {
	BatchGetImageWithContext(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error)
	GetDownloadUrlForLayerWithContext(aws.Context, *ecr.GetDownloadUrlForLayerInput, ...request.Option) (*ecr.GetDownloadUrlForLayerOutput, error)
	BatchCheckLayerAvailabilityWithContext(aws.Context, *ecr.BatchCheckLayerAvailabilityInput, ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error)
	InitiateLayerUpload(*ecr.InitiateLayerUploadInput) (*ecr.InitiateLayerUploadOutput, error)
	UploadLayerPart(*ecr.UploadLayerPartInput) (*ecr.UploadLayerPartOutput, error)
	CompleteLayerUpload(*ecr.CompleteLayerUploadInput) (*ecr.CompleteLayerUploadOutput, error)
	PutImageWithContext(aws.Context, *ecr.PutImageInput, ...request.Option) (*ecr.PutImageOutput, error)
}

func (b *ecrBase) getManifest(ctx context.Context) (*ecr.Image, error) {
	imageIdentifier := b.ecrSpec.ImageID()
	log.G(ctx).WithField("imageIdentifier", imageIdentifier).Debug("ecr.base.manifest")
	batchGetImageInput := &ecr.BatchGetImageInput{
		RegistryId:     aws.String(b.ecrSpec.Registry()),
		RepositoryName: aws.String(b.ecrSpec.Repository),
		ImageIds:       []*ecr.ImageIdentifier{imageIdentifier},
		// TODO: Determine if this should be hard-coded
		AcceptedMediaTypes: []*string{
			aws.String(ocispec.MediaTypeImageManifest),
			aws.String(images.MediaTypeDockerSchema2Manifest),
		},
	}

	batchGetImageOutput, err := b.client.BatchGetImageWithContext(ctx, batchGetImageInput)
	if err != nil {
		log.G(ctx).WithError(err).Error("ecr.base.manifest: failed to get image")
		fmt.Println(err)
		return nil, err
	}
	log.G(ctx).WithField("batchGetImage", batchGetImageOutput).Debug("ecr.base.manifest")

	var ecrImage *ecr.Image
	if len(batchGetImageOutput.Images) == 0 {
		if len(batchGetImageOutput.Failures) > 0 &&
			aws.StringValue(batchGetImageOutput.Failures[0].FailureCode) == ecr.ImageFailureCodeImageNotFound {
			return nil, errImageNotFound
		}
		log.G(ctx).
			WithField("failures", batchGetImageOutput.Failures).
			Warn("ecr.base.manifest: unexpected failure")
		return nil, reference.ErrInvalid
	}
	ecrImage = batchGetImageOutput.Images[0]
	return ecrImage, nil
}
