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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestWriterCommit(t *testing.T) {
	const (
		manifestContent = "manifest content"
		registry        = "registry"
		repository      = "repository"
		imageTag        = "tag"
	)

	// Setup an image details for push.
	imageDigest := testdata.InsignificantDigest
	imageDesc := ocispec.Descriptor{
		Digest:    imageDigest,
		MediaType: ocispec.MediaTypeImageManifest,
	}
	// root image Object has its digest appended.
	imageObject := imageTag + "@" + imageDigest.String()
	imageECRSpec := ECRSpec{
		arn: arn.ARN{
			AccountID: registry,
		},
		Repository: repository,
		Object:     imageObject,
	}
	// root image ref is the ECRSpec's formatted ref, with the digest of the
	// root descriptor. For a single manifest image, that's the manifest's
	// digest.
	refKey := imageECRSpec.Canonical()

	t.Log("image Object: ", imageObject)
	t.Log("image digest: ", imageDigest)

	callCount := 0
	client := &fakeECRClient{
		PutImageFn: func(_ context.Context, input *ecr.PutImageInput, _ ...func(*ecr.Options)) (*ecr.PutImageOutput, error) {
			callCount++

			assert.Equal(t, registry, aws.ToString(input.RegistryId))
			assert.Equal(t, repository, aws.ToString(input.RepositoryName))
			assert.Equal(t, imageTag, aws.ToString(input.ImageTag),
				"should use image ref's tag")
			assert.Equal(t, manifestContent, aws.ToString(input.ImageManifest),
				"should provide manifest's body")
			assert.Equal(t, imageDesc.MediaType, aws.ToString(input.ImageManifestMediaType),
				"should include manifest's mediaType in API input") // regardless of it being in the manifest body
			assert.Equal(t, imageDesc.Digest.String(), aws.ToString(input.ImageDigest),
				"should include manifest's digest in API input")

			return &ecr.PutImageOutput{
				Image: &types.Image{
					ImageId: &types.ImageIdentifier{
						ImageTag:    input.ImageTag,
						ImageDigest: aws.String(imageDigest.String()),
					},
				},
			}, nil
		},
	}
	mw := &manifestWriter{
		desc: imageDesc,
		base: &ecrBase{
			client:  client,
			ecrSpec: imageECRSpec,
		},
		tracker: docker.NewInMemoryTracker(),
		ref:     refKey,
		ctx:     context.Background(),
	}

	count, err := mw.Write([]byte(manifestContent[:3]))
	require.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, 3, count, "wrong number of bytes")

	count, err = mw.Write([]byte(manifestContent[3:]))
	require.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, len(manifestContent)-3, count, "wrong number of bytes")

	assert.Equal(t, 0, callCount, "PutImage should not be called until committed")

	err = mw.Commit(context.Background(), int64(len(manifestContent)), imageDigest)
	require.NoError(t, err, "failed to commit")
	assert.Equal(t, 1, callCount, "PutImage should be called once")
}

func TestManifestWriterNoTagCommit(t *testing.T) {
	const (
		registry   = "registry"
		repository = "repository"
		imageTag   = "tag"

		memberManifestContent = "manifest content"
	)

	// The root image, this is the target digest which is treated as an Image
	// Index in this test case.
	imageDigest := testdata.ImageDigest
	// Image pushes include the root image digest in the object:
	//
	// ie: "latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	imageObject := imageTag + "@" + imageDigest.String()

	// A member manifest that was listed and being "pushed" in the test case.
	memberDesc := ocispec.Descriptor{
		Digest:    "member-digest",
		MediaType: ocispec.MediaTypeImageManifest,
	}
	// ref, for non-root descriptors, uses the internal ref naming (eg:
	// index-sha256:fffff...).
	refKey := remotes.MakeRefKey(context.Background(), memberDesc)

	t.Log("image Object: ", imageObject)
	t.Log("image digest: " + imageDigest.String())
	t.Log("member digest: " + memberDesc.Digest.String())

	callCount := 0
	client := &fakeECRClient{
		PutImageFn: func(_ context.Context, input *ecr.PutImageInput, _ ...func(*ecr.Options)) (*ecr.PutImageOutput, error) {
			callCount++
			assert.Equal(t, registry, aws.ToString(input.RegistryId))
			assert.Equal(t, repository, aws.ToString(input.RepositoryName))
			assert.NotEqual(t, aws.ToString(input.ImageTag), imageTag, "should not include tag when pushing non-root descriptor")
			assert.Equal(t, memberManifestContent, aws.ToString(input.ImageManifest),
				"should provide manifest's body")
			assert.Equal(t, memberDesc.MediaType, aws.ToString(input.ImageManifestMediaType),
				"should include manifest's mediaType in API input") // regardless of it being in the manifest body
			assert.Equal(t, memberDesc.Digest.String(), aws.ToString(input.ImageDigest),
				"should include manifest's digest in API input")

			return &ecr.PutImageOutput{
				Image: &types.Image{
					ImageId: &types.ImageIdentifier{
						// Image will have the matching digest.
						ImageDigest: aws.String(memberDesc.Digest.String()),
					},
				},
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
				Object:     imageObject,
			},
		},
		desc:    memberDesc,
		tracker: docker.NewInMemoryTracker(),
		ref:     refKey,
		ctx:     context.Background(),
	}

	count, err := mw.Write([]byte(memberManifestContent[:3]))
	require.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, 3, count, "wrong number of bytes")

	count, err = mw.Write([]byte(memberManifestContent[3:]))
	require.NoError(t, err, "failed to write to manifest writer")
	assert.Equal(t, len(memberManifestContent)-3, count, "wrong number of bytes")

	assert.Equal(t, 0, callCount, "PutImage should not be called until committed")

	err = mw.Commit(context.Background(), int64(len(memberManifestContent)), memberDesc.Digest)
	require.NoError(t, err, "failed to commit")
	assert.Equal(t, 1, callCount, "PutImage should be called once")
}
