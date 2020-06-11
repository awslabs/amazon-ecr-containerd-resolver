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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushManifestReturnsManifestWriter(t *testing.T) {
	registry := "registry"
	repository := "repository"
	imageTag := "tag"
	imageDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{}
	pusher := &ecrPusher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
				Object:     imageTag,
			},
		},
		tracker: docker.NewInMemoryTracker(),
	}

	// test all supported media types
	for _, mediaType := range []string{
		ocispec.MediaTypeImageManifest,
		images.MediaTypeDockerSchema2Manifest,
		images.MediaTypeDockerSchema1Manifest,
	} {
		t.Run(mediaType, func(t *testing.T) {
			callCount := 0
			fakeClient.BatchGetImageFn = func(_ aws.Context, input *ecr.BatchGetImageInput, _ ...request.Option) (*ecr.BatchGetImageOutput, error) {
				callCount++
				assert.Equal(t, registry, aws.StringValue(input.RegistryId))
				assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
				assert.Equal(t, []*ecr.ImageIdentifier{{ImageTag: aws.String(imageTag)}}, input.ImageIds)
				// TODO: Determine if we should be matching the requested media type from containerd
				assert.Equal(t, []*string{
					aws.String(ocispec.MediaTypeImageManifest),
					aws.String(images.MediaTypeDockerSchema2Manifest),
				}, input.AcceptedMediaTypes)
				return &ecr.BatchGetImageOutput{
					Failures: []*ecr.ImageFailure{
						{FailureCode: aws.String(ecr.ImageFailureCodeImageNotFound)},
					},
				}, nil
			}
			desc := ocispec.Descriptor{
				MediaType: mediaType,
				Digest:    digest.Digest(imageDigest),
			}

			start := time.Now()
			writer, err := pusher.Push(context.Background(), desc)
			assert.Equal(t, 1, callCount, "BatchGetImage should be called once")
			assert.NoError(t, err)
			_, ok := writer.(*manifestWriter)
			assert.True(t, ok, "writer should be a manifestWriter")
			end := time.Now()
			writer.Close()

			refKey := remotes.MakeRefKey(context.Background(), desc)
			status, err := pusher.tracker.GetStatus(refKey)
			assert.NoError(t, err, "should retrieve status")
			assert.WithinDuration(t,
				start,
				status.Status.StartedAt,
				end.Sub(start),
				"should be updated between start and end")
		})
	}
}

func TestPushManifestAlreadyExists(t *testing.T) {
	registry := "registry"
	repository := "repository"
	imageTag := "tag"
	imageDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error) {
			return &ecr.BatchGetImageOutput{
				Images: []*ecr.Image{
					{ImageId: &ecr.ImageIdentifier{ImageDigest: aws.String(imageDigest)}},
				},
			}, nil
		},
	}
	pusher := &ecrPusher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
				Object:     imageTag,
			},
		},
		tracker: docker.NewInMemoryTracker(),
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.Digest(imageDigest),
	}

	start := time.Now()
	_, err := pusher.Push(context.Background(), desc)
	assert.Error(t, err)
	cause := errors.Cause(err)
	assert.Equal(t, errdefs.ErrAlreadyExists, cause)
	end := time.Now()

	refKey := remotes.MakeRefKey(context.Background(), desc)
	status, err := pusher.tracker.GetStatus(refKey)
	assert.NoError(t, err, "should retrieve status")
	assert.WithinDuration(t,
		start,
		status.Status.UpdatedAt,
		end.Sub(start),
		"should be updated between start and end")
}

func TestPushBlobReturnsLayerWriter(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{
		InitiateLayerUploadFn: func(*ecr.InitiateLayerUploadInput) (*ecr.InitiateLayerUploadOutput, error) {
			// layerWriter calls this during its constructor
			return &ecr.InitiateLayerUploadOutput{}, nil
		},
	}
	pusher := &ecrPusher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
		tracker: docker.NewInMemoryTracker(),
	}

	// test all supported media types
	for _, mediaType := range []string{
		images.MediaTypeDockerSchema2Layer,
		images.MediaTypeDockerSchema2LayerGzip,
		images.MediaTypeDockerSchema2Config,
		ocispec.MediaTypeImageLayerGzip,
		ocispec.MediaTypeImageLayer,
		ocispec.MediaTypeImageConfig,
	} {
		t.Run(mediaType, func(t *testing.T) {
			callCount := 0
			fakeClient.BatchCheckLayerAvailabilityFn = func(_ aws.Context, input *ecr.BatchCheckLayerAvailabilityInput, _ ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error) {
				callCount++
				assert.Equal(t, registry, aws.StringValue(input.RegistryId))
				assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
				require.Len(t, input.LayerDigests, 1)
				assert.Equal(t, layerDigest, aws.StringValue(input.LayerDigests[0]))
				return &ecr.BatchCheckLayerAvailabilityOutput{
					Layers: []*ecr.Layer{{
						LayerAvailability: aws.String(ecr.LayerAvailabilityUnavailable),
					}},
				}, nil
			}

			desc := ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageLayerGzip,
				Digest:    digest.Digest(layerDigest),
			}

			start := time.Now()
			writer, err := pusher.Push(context.Background(), desc)
			assert.Equal(t, 1, callCount, "BatchCheckLayerAvailability should be called once")
			assert.NoError(t, err)
			_, ok := writer.(*layerWriter)
			assert.True(t, ok, "writer should be a layerWriter")
			end := time.Now()
			writer.Close()

			refKey := remotes.MakeRefKey(context.Background(), desc)
			status, err := pusher.tracker.GetStatus(refKey)
			assert.NoError(t, err, "should retrieve status")
			assert.WithinDuration(t,
				start,
				status.Status.StartedAt,
				end.Sub(start),
				"should be updated between start and end")
		})
	}
}

func TestPushBlobAlreadyExists(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{
		BatchCheckLayerAvailabilityFn: func(aws.Context, *ecr.BatchCheckLayerAvailabilityInput, ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error) {
			return &ecr.BatchCheckLayerAvailabilityOutput{
				Layers: []*ecr.Layer{{
					LayerAvailability: aws.String(ecr.LayerAvailabilityAvailable),
				}},
			}, nil
		},
	}
	pusher := &ecrPusher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
		tracker: docker.NewInMemoryTracker(),
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    digest.Digest(layerDigest),
	}

	start := time.Now()
	_, err := pusher.Push(context.Background(), desc)
	assert.Error(t, err)
	cause := errors.Cause(err)
	assert.Equal(t, errdefs.ErrAlreadyExists, cause)
	end := time.Now()

	refKey := remotes.MakeRefKey(context.Background(), desc)
	status, err := pusher.tracker.GetStatus(refKey)
	assert.NoError(t, err, "should retrieve status")
	assert.WithinDuration(t,
		start,
		status.Status.UpdatedAt,
		end.Sub(start),
		"should be updated between start and end")
}

func TestPushBlobAPIError(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{
		BatchCheckLayerAvailabilityFn: func(aws.Context, *ecr.BatchCheckLayerAvailabilityInput, ...request.Option) (*ecr.BatchCheckLayerAvailabilityOutput, error) {
			return &ecr.BatchCheckLayerAvailabilityOutput{
				Failures: []*ecr.LayerFailure{{}},
			}, nil
		},
	}
	pusher := &ecrPusher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
		tracker: docker.NewInMemoryTracker(),
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    digest.Digest(layerDigest),
	}

	_, err := pusher.Push(context.Background(), desc)
	assert.EqualError(t, err, errLayerNotFound.Error())
}
