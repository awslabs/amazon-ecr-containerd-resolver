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
	"io"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestLayerWriter(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerData := "layer"
	layerDigest := testdata.InsignificantDigest.String()
	uploadID := "upload"
	initiateLayerUploadCount, uploadLayerPartCount, completeLayerUploadCount := 0, 0, 0
	client := &fakeECRClient{
		InitiateLayerUploadFn: func(input *ecr.InitiateLayerUploadInput) (*ecr.InitiateLayerUploadOutput, error) {
			initiateLayerUploadCount++
			assert.Equal(t, registry, aws.StringValue(input.RegistryId))
			assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
			return &ecr.InitiateLayerUploadOutput{
				UploadId: aws.String(uploadID),
				// use single-byte upload size so we can test each byte
				PartSize: aws.Int64(1),
			}, nil
		},
		UploadLayerPartFn: func(input *ecr.UploadLayerPartInput) (*ecr.UploadLayerPartOutput, error) {
			assert.Equal(t, registry, aws.StringValue(input.RegistryId))
			assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
			assert.Equal(t, uploadID, aws.StringValue(input.UploadId))
			assert.Equal(t, int64(uploadLayerPartCount), aws.Int64Value(input.PartFirstByte), "first byte")
			assert.Equal(t, int64(uploadLayerPartCount), aws.Int64Value(input.PartLastByte), "last byte")
			assert.Len(t, input.LayerPartBlob, 1, "only one byte is expected")
			assert.Equal(t, layerData[uploadLayerPartCount], input.LayerPartBlob[0], "invalid layer blob data")
			uploadLayerPartCount++
			return nil, nil
		},
		CompleteLayerUploadFn: func(input *ecr.CompleteLayerUploadInput) (*ecr.CompleteLayerUploadOutput, error) {
			completeLayerUploadCount++
			assert.Equal(t, registry, aws.StringValue(input.RegistryId))
			assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
			assert.Equal(t, uploadID, aws.StringValue(input.UploadId))
			assert.Equal(t, len(layerData), uploadLayerPartCount)
			return &ecr.CompleteLayerUploadOutput{
				LayerDigest: aws.String(layerDigest),
			}, nil
		},
	}
	ecrBase := &ecrBase{
		client: client,
		ecrSpec: ECRSpec{
			arn: arn.ARN{
				AccountID: registry,
			},
			Repository: repository,
		},
	}

	desc := ocispec.Descriptor{
		Digest: digest.Digest(layerDigest),
	}

	tracker := docker.NewInMemoryTracker()
	refKey := "refKey"
	tracker.SetStatus(refKey, docker.Status{})

	lw, err := newLayerWriter(ecrBase, tracker, "refKey", desc)
	assert.NoError(t, err)
	assert.Equal(t, 1, initiateLayerUploadCount)
	assert.Equal(t, 0, uploadLayerPartCount)
	assert.Equal(t, 0, completeLayerUploadCount)

	n, err := lw.Write([]byte(layerData))
	assert.NoError(t, err)
	assert.Equal(t, len(layerData), n)

	err = lw.Commit(context.TODO(), int64(len(layerData)), desc.Digest)
	assert.NoError(t, err)
	assert.Equal(t, 1, completeLayerUploadCount)
}

type layerAlreadyExistsError struct{}

func (l *layerAlreadyExistsError) Code() string    { return "LayerAlreadyExistsException" }
func (l *layerAlreadyExistsError) Error() string   { return l.Code() }
func (l *layerAlreadyExistsError) Message() string { return l.Code() }
func (l *layerAlreadyExistsError) OrigErr() error  { return l }

var _ awserr.Error = (*layerAlreadyExistsError)(nil)

func TestLayerWriterCommitExists(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := "sha256:digest"
	callCount := 0
	client := &fakeECRClient{
		CompleteLayerUploadFn: func(_ *ecr.CompleteLayerUploadInput) (*ecr.CompleteLayerUploadOutput, error) {
			callCount++
			return nil, &layerAlreadyExistsError{}
		},
	}

	_, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lw := layerWriter{
		base: &ecrBase{
			client: client,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
		buf: writer,
		ctx: ctx,
	}

	err := lw.Commit(context.Background(), 0, digest.Digest(layerDigest))
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}
