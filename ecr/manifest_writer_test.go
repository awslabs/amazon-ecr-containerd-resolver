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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestManifestWriterCommit(t *testing.T) {
	manifestContent := "manifest content"
	registry := "registry"
	repository := "repository"
	imageTag := "tag"
	imageDigest := testdata.InsignificantDigest.String()
	refKey := "refKey"
	callCount := 0
	client := &fakeECRClient{
		PutImageFn: func(_ aws.Context, input *ecr.PutImageInput, _ ...request.Option) (*ecr.PutImageOutput, error) {
			callCount++
			assert.Equal(t, registry, aws.StringValue(input.RegistryId))
			assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
			assert.Equal(t, imageTag, aws.StringValue(input.ImageTag))
			assert.Equal(t, manifestContent, aws.StringValue(input.ImageManifest))
			return &ecr.PutImageOutput{
				Image: &ecr.Image{ImageId: &ecr.ImageIdentifier{ImageDigest: aws.String(imageDigest)}},
			}, nil
		},
	}
	mw := &manifestWriter{
		base: &ecrBase{
			client: client,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
				Object:     imageTag,
			},
		},
		tracker: docker.NewInMemoryTracker(),
		ref:     refKey,
		ctx:     context.Background(),
	}

	count, err := mw.Write([]byte(manifestContent[:3]))
	assert.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, 3, count, "wrong number of bytes")

	count, err = mw.Write([]byte(manifestContent[3:]))
	assert.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, len(manifestContent)-3, count, "wrong number of bytes")

	assert.Equal(t, 0, callCount, "PutImage should not be called until committed")

	err = mw.Commit(context.Background(), int64(len(manifestContent)), digest.Digest(imageDigest))
	assert.NoError(t, err, "failed to commit")
	assert.Equal(t, 1, callCount, "PutImage should be called once")
}
