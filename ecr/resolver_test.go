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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"

	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
)

func TestParseImageManifestMediaType(t *testing.T) {
	cases := []struct {
		name      string
		manifest  string
		mediaType string
	}{
		{
			name:      "default",
			manifest:  "",
			mediaType: images.MediaTypeDockerSchema2Manifest,
		},
		{
			name:      "schemaVersion:1 unsigned",
			manifest:  `{"schemaVersion": 1}`,
			mediaType: "application/vnd.docker.distribution.manifest.v1+json",
		},
		{
			name:      "schemaVersion:1",
			manifest:  `{"schemaVersion": 1, "signatures":[{}]}`,
			mediaType: images.MediaTypeDockerSchema1Manifest,
		},
		{
			name:      "schemaVersion:2 docker",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.v2+json"}`,
			mediaType: images.MediaTypeDockerSchema2Manifest,
		},
		{
			name:      "schemaVersion:2 docker manifest list",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json", "manifests": []}`,
			mediaType: images.MediaTypeDockerSchema2ManifestList,
		},
		{
			name:      "schemaVersion:2 oci",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json"}`,
			mediaType: ocispec.MediaTypeImageManifest,
		},
		{
			name:      "schemaVersion:2 oci manifest list",
			manifest:  `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": []}`,
			mediaType: ocispec.MediaTypeImageIndex,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mediaType := parseImageManifestMediaType(context.TODO(), tc.manifest)
			assert.Equal(t, tc.mediaType, mediaType)
		})
	}
}

func TestResolve(t *testing.T) {
	// input
	expectedRef := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"

	// expected API arguments
	expectedRegistryID := "123456789012"
	expectedRepository := "foo/bar"
	expectedImageTag := "latest"

	// API output
	imageDigest := testdata.ImageDigest.String()
	imageManifest := `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json"}`
	image := &ecr.Image{
		RepositoryName: aws.String(expectedRepository),
		ImageId: &ecr.ImageIdentifier{
			ImageDigest: aws.String(imageDigest),
		},
		ImageManifest: aws.String(imageManifest),
	}

	// expected output
	expectedDesc := ocispec.Descriptor{
		Digest:    digest.Digest(imageDigest),
		MediaType: ocispec.MediaTypeImageManifest,
		Size:      int64(len(imageManifest)),
	}

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(ctx aws.Context, input *ecr.BatchGetImageInput, opts ...request.Option) (*ecr.BatchGetImageOutput, error) {
			assert.Equal(t, expectedRegistryID, aws.StringValue(input.RegistryId))
			assert.Equal(t, expectedRepository, aws.StringValue(input.RepositoryName))
			assert.Equal(t, []*ecr.ImageIdentifier{{ImageTag: aws.String(expectedImageTag)}}, input.ImageIds)
			return &ecr.BatchGetImageOutput{Images: []*ecr.Image{image}}, nil
		},
	}
	resolver := &ecrResolver{
		clients: map[string]ecrAPI{
			"fake": fakeClient,
		},
	}

	ref, desc, err := resolver.Resolve(context.Background(), expectedRef)
	assert.NoError(t, err)
	assert.Equal(t, expectedRef, ref)
	assert.Equal(t, expectedDesc, desc)
}

func TestResolveError(t *testing.T) {
	// input
	ref := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"

	// expected output
	expectedError := errors.New("expected")

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error) {
			return nil, expectedError
		},
	}
	resolver := &ecrResolver{
		clients: map[string]ecrAPI{
			"fake": fakeClient,
		},
	}
	_, _, err := resolver.Resolve(context.Background(), ref)
	assert.EqualError(t, err, expectedError.Error())
}

func TestResolveNoResult(t *testing.T) {
	// input
	ref := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error) {
			return &ecr.BatchGetImageOutput{}, nil
		},
	}
	resolver := &ecrResolver{
		clients: map[string]ecrAPI{
			"fake": fakeClient,
		},
	}
	_, _, err := resolver.Resolve(context.Background(), ref)
	assert.Error(t, err)
	assert.Equal(t, reference.ErrInvalid, err)
}

func TestResolvePusherDenyDigest(t *testing.T) {
	for _, ref := range []string{
		"ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar@" + testdata.ImageDigest.String(),
	} {
		t.Run(ref, func(t *testing.T) {
			resolver := &ecrResolver{}
			p, err := resolver.Pusher(context.Background(), ref)
			assert.Error(t, err)
			assert.Nil(t, p)
		})
	}

}
