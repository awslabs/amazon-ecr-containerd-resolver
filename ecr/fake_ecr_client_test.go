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

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// fakeECRClient is a fake that can be used for testing the ecrAPI interface.
// Each method is backed by a function contained in the struct.  Nil functions
// will cause panics when invoked.
type fakeECRClient struct {
	BatchGetImageFn               func(context.Context, *ecr.BatchGetImageInput, ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error)
	GetDownloadUrlForLayerFn      func(context.Context, *ecr.GetDownloadUrlForLayerInput, ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error)
	BatchCheckLayerAvailabilityFn func(context.Context, *ecr.BatchCheckLayerAvailabilityInput, ...func(*ecr.Options)) (*ecr.BatchCheckLayerAvailabilityOutput, error)
	InitiateLayerUploadFn         func(context.Context, *ecr.InitiateLayerUploadInput, ...func(*ecr.Options)) (*ecr.InitiateLayerUploadOutput, error)
	UploadLayerPartFn             func(context.Context, *ecr.UploadLayerPartInput, ...func(*ecr.Options)) (*ecr.UploadLayerPartOutput, error)
	CompleteLayerUploadFn         func(context.Context, *ecr.CompleteLayerUploadInput, ...func(*ecr.Options)) (*ecr.CompleteLayerUploadOutput, error)
	PutImageFn                    func(context.Context, *ecr.PutImageInput, ...func(*ecr.Options)) (*ecr.PutImageOutput, error)
}

var _ ecrAPI = (*fakeECRClient)(nil)

func (f *fakeECRClient) BatchGetImage(ctx context.Context, arg *ecr.BatchGetImageInput, opts ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error) {
	return f.BatchGetImageFn(ctx, arg, opts...)
}

func (f *fakeECRClient) GetDownloadUrlForLayer(ctx context.Context, arg *ecr.GetDownloadUrlForLayerInput, opts ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error) {
	return f.GetDownloadUrlForLayerFn(ctx, arg, opts...)
}

func (f *fakeECRClient) BatchCheckLayerAvailability(ctx context.Context, arg *ecr.BatchCheckLayerAvailabilityInput, opts ...func(*ecr.Options)) (*ecr.BatchCheckLayerAvailabilityOutput, error) {
	return f.BatchCheckLayerAvailabilityFn(ctx, arg, opts...)
}

func (f *fakeECRClient) InitiateLayerUpload(ctx context.Context, arg *ecr.InitiateLayerUploadInput, opts ...func(*ecr.Options)) (*ecr.InitiateLayerUploadOutput, error) {
	return f.InitiateLayerUploadFn(ctx, arg, opts...)
}

func (f *fakeECRClient) UploadLayerPart(ctx context.Context, arg *ecr.UploadLayerPartInput, opts ...func(*ecr.Options)) (*ecr.UploadLayerPartOutput, error) {
	return f.UploadLayerPartFn(ctx, arg, opts...)
}

func (f *fakeECRClient) CompleteLayerUpload(ctx context.Context, arg *ecr.CompleteLayerUploadInput, opts ...func(*ecr.Options)) (*ecr.CompleteLayerUploadOutput, error) {
	return f.CompleteLayerUploadFn(ctx, arg, opts...)
}

func (f *fakeECRClient) PutImage(ctx context.Context, arg *ecr.PutImageInput, opts ...func(*ecr.Options)) (*ecr.PutImageOutput, error) {
	return f.PutImageFn(ctx, arg, opts...)
}
