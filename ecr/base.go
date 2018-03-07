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
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
)

var (
	errImageNotFound = errors.New("ecr: image not found")
)

type ecrBase struct {
	client  ecriface.ECRAPI
	ecrSpec ECRSpec
}

func (f *ecrBase) getManifest(ctx context.Context, imageIdentifier *ecr.ImageIdentifier) (*ecr.Image, error) {
	fmt.Printf("getManifest: imageIdentifier=%v\n", imageIdentifier)
	batchGetImageInput := &ecr.BatchGetImageInput{
		RegistryId:         aws.String(f.ecrSpec.Registry()),
		RepositoryName:     aws.String(f.ecrSpec.Repository),
		ImageIds:           []*ecr.ImageIdentifier{imageIdentifier},
		AcceptedMediaTypes: []*string{aws.String(images.MediaTypeDockerSchema2Manifest)},
	}

	batchGetImageOutput, err := f.client.BatchGetImage(batchGetImageInput)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(batchGetImageOutput)

	var ecrImage *ecr.Image
	if len(batchGetImageOutput.Images) != 1 {
		if len(batchGetImageOutput.Failures) > 0 &&
			aws.StringValue(batchGetImageOutput.Failures[0].FailureCode) == ecr.ImageFailureCodeImageNotFound {
			return nil, errImageNotFound
		}
		fmt.Println("what")
		return nil, reference.ErrInvalid
	}
	ecrImage = batchGetImageOutput.Images[0]
	return ecrImage, nil
}