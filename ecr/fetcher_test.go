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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchUnimplemented(t *testing.T) {
	fetcher := &ecrFetcher{}
	desc := ocispec.Descriptor{
		MediaType: "never-implemented",
	}
	_, err := fetcher.Fetch(context.Background(), desc)
	assert.EqualError(t, err, unimplemented.Error())
}

func TestFetchForeignLayer(t *testing.T) {
	// setup
	expectedBody := "hello this is dog"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expectedBody)
	}))
	defer ts.Close()

	fetcher := &ecrFetcher{}

	// test both media types
	for _, mediaType := range []string{
		images.MediaTypeDockerSchema2LayerForeign,
		images.MediaTypeDockerSchema2LayerForeignGzip,
	} {
		t.Run(mediaType, func(t *testing.T) {
			// input
			desc := ocispec.Descriptor{
				MediaType: mediaType,
				URLs:      []string{ts.URL},
			}

			reader, err := fetcher.Fetch(context.Background(), desc)
			require.NoError(t, err, "fetch should succeed from test server")
			defer reader.Close()

			output, err := ioutil.ReadAll(reader)
			assert.NoError(t, err, "should have a valid byte buffer")
			assert.Equal(t, expectedBody, string(output))
		})
	}
}

func TestFetchForeignLayerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	fetcher := &ecrFetcher{}
	mediaType := images.MediaTypeDockerSchema2LayerForeignGzip

	desc := ocispec.Descriptor{
		MediaType: mediaType,
		URLs:      []string{ts.URL},
	}

	_, err := fetcher.Fetch(context.Background(), desc)
	assert.Error(t, err)
	cause := errors.Cause(err)
	assert.Equal(t, errdefs.ErrNotFound, cause)
}

func TestFetchManifest(t *testing.T) {
	registry := "registry"
	repository := "repository"
	imageTag := "tag"
	imageManifest := "image manifest"
	fakeClient := &fakeECRClient{}
	fetcher := &ecrFetcher{
		ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
				Object:     imageTag,
			},
		},
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
				assert.Equal(t, []*string{aws.String(images.MediaTypeDockerSchema2Manifest)}, input.AcceptedMediaTypes)
				return &ecr.BatchGetImageOutput{Images: []*ecr.Image{{ImageManifest: aws.String(imageManifest)}}}, nil
			}
			desc := ocispec.Descriptor{
				MediaType: mediaType,
			}
			reader, err := fetcher.Fetch(context.Background(), desc)
			assert.NoError(t, err, "fetch")
			defer reader.Close()
			assert.Equal(t, 1, callCount, "BatchGetImage should be called once")
			manifest, err := ioutil.ReadAll(reader)
			assert.NoError(t, err, "reading manifest")
			assert.Equal(t, imageManifest, string(manifest))
		})
	}
}

func TestFetchManifestAPIError(t *testing.T) {
	ref := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"
	mediaType := ocispec.MediaTypeImageManifest

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error) {
			return nil, errors.New("expected")
		},
	}
	resolver := &ecrResolver{
		clients: map[string]ecrAPI{
			"fake": fakeClient,
		},
	}
	fetcher, err := resolver.Fetcher(context.Background(), ref)
	require.NoError(t, err, "failed to create fetcher")
	_, err = fetcher.Fetch(context.Background(), ocispec.Descriptor{MediaType: mediaType})
	assert.EqualError(t, err, "expected")
}

func TestFetchManifestNotFound(t *testing.T) {
	ref := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"
	mediaType := ocispec.MediaTypeImageManifest

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(aws.Context, *ecr.BatchGetImageInput, ...request.Option) (*ecr.BatchGetImageOutput, error) {
			return &ecr.BatchGetImageOutput{
				Failures: []*ecr.ImageFailure{
					{FailureCode: aws.String(ecr.ImageFailureCodeImageNotFound)},
				},
			}, nil
		},
	}
	resolver := &ecrResolver{
		clients: map[string]ecrAPI{
			"fake": fakeClient,
		},
	}
	fetcher, err := resolver.Fetcher(context.Background(), ref)
	require.NoError(t, err, "failed to create fetcher")
	_, err = fetcher.Fetch(context.Background(), ocispec.Descriptor{MediaType: mediaType})
	assert.Error(t, err)
}

func TestFetchLayer(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := "digest"
	fakeClient := &fakeECRClient{}
	fetcher := &ecrFetcher{
		ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
	}
	expectedBody := "hello this is dog"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expectedBody)
	}))
	defer ts.Close()

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
			fakeClient.GetDownloadUrlForLayerFn = func(_ aws.Context, input *ecr.GetDownloadUrlForLayerInput, _ ...request.Option) (*ecr.GetDownloadUrlForLayerOutput, error) {
				callCount++
				assert.Equal(t, registry, aws.StringValue(input.RegistryId))
				assert.Equal(t, repository, aws.StringValue(input.RepositoryName))
				assert.Equal(t, layerDigest, aws.StringValue(input.LayerDigest))
				return &ecr.GetDownloadUrlForLayerOutput{DownloadUrl: aws.String(ts.URL)}, nil
			}
			desc := ocispec.Descriptor{
				MediaType: mediaType,
				Digest:    digest.Digest(layerDigest),
			}
			reader, err := fetcher.Fetch(context.Background(), desc)
			assert.NoError(t, err, "fetch")
			defer reader.Close()
			assert.Equal(t, 1, callCount, "GetDownloadURLForLayer should be called once")
			body, err := ioutil.ReadAll(reader)
			assert.NoError(t, err, "reading body")
			assert.Equal(t, expectedBody, string(body))
		})
	}
}

func TestFetchLayerAPIError(t *testing.T) {
	fakeClient := &fakeECRClient{
		GetDownloadUrlForLayerFn: func(aws.Context, *ecr.GetDownloadUrlForLayerInput, ...request.Option) (*ecr.GetDownloadUrlForLayerOutput, error) {
			return nil, errors.New("expected")
		},
	}
	fetcher := &ecrFetcher{
		ecrBase{
			client: fakeClient,
		},
	}
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
	}
	_, err := fetcher.Fetch(context.Background(), desc)
	assert.Error(t, err)
}
