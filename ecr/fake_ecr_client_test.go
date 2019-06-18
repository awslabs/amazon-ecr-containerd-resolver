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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
)

// fakeECRClient is a fake that can be used for testing the ecrAPI interface.
// Each method is backed by a function contained in the struct.  Nil functions
// will cause panics when invoked.
type fakeECRClient struct {
	BatchGetImageFn               func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error)
	GetDownloadUrlForLayerFn      func(aws.Context, *ecr.GetDownloadUrlForLayerInput, ...request.Option) (*ecr.GetDownloadUrlForLayerOutput, error)
	BatchCheckLayerAvailabilityFn func(aws.Context, *ecr.BatchCheckLayerAvailabilityInput, ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error)
	InitiateLayerUploadFn         func(*ecr.InitiateLayerUploadInput) (*ecr.InitiateLayerUploadOutput, error)
	UploadLayerPartFn             func(*ecr.UploadLayerPartInput) (*ecr.UploadLayerPartOutput, error)
	CompleteLayerUploadFn         func(*ecr.CompleteLayerUploadInput) (*ecr.CompleteLayerUploadOutput, error)
	PutImageFn                    func(aws.Context, *ecr.PutImageInput, ...request.Option) (*ecr.PutImageOutput, error)
}

var _ ecrAPI = (*fakeECRClient)(nil)

func (f *fakeECRClient) BatchGetImageWithContext(ctx aws.Context, arg *ecr.BatchGetImageInput, opts ...request.Option) (*ecr.BatchGetImageOutput, error) {
	return f.BatchGetImageFn(ctx, arg, opts...)
}

func (f *fakeECRClient) GetDownloadUrlForLayerWithContext(ctx aws.Context, arg *ecr.GetDownloadUrlForLayerInput, opts ...request.Option) (*ecr.GetDownloadUrlForLayerOutput, error) {
	return f.GetDownloadUrlForLayerFn(ctx, arg, opts...)
}

func (f *fakeECRClient) BatchCheckLayerAvailabilityWithContext(ctx aws.Context, arg *ecr.BatchCheckLayerAvailabilityInput, opts ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error) {
	return f.BatchCheckLayerAvailabilityFn(ctx, arg, opts...)
}

func (f *fakeECRClient) InitiateLayerUpload(arg *ecr.InitiateLayerUploadInput) (*ecr.InitiateLayerUploadOutput, error) {
	return f.InitiateLayerUploadFn(arg)
}

func (f *fakeECRClient) UploadLayerPart(arg *ecr.UploadLayerPartInput) (*ecr.UploadLayerPartOutput, error) {
	return f.UploadLayerPartFn(arg)
}

func (f *fakeECRClient) CompleteLayerUpload(arg *ecr.CompleteLayerUploadInput) (*ecr.CompleteLayerUploadOutput, error) {
	return f.CompleteLayerUploadFn(arg)
}

func (f *fakeECRClient) PutImageWithContext(ctx aws.Context, arg *ecr.PutImageInput, opts ...request.Option) (*ecr.PutImageOutput, error) {
	return f.PutImageFn(ctx, arg, opts...)
}
