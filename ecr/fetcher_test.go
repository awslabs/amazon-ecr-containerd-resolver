/*
 * Copyright 2017-2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	const expectedBody = "hello, this is dog"

	fetcher := &ecrFetcher{}

	// test both media types
	for _, mediaType := range []string{
		images.MediaTypeDockerSchema2LayerForeign,
		images.MediaTypeDockerSchema2LayerForeignGzip,
	} {
		t.Run(mediaType, func(t *testing.T) {
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path == "/missing" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Fprint(w, expectedBody)
			}))
			defer ts.Close()

			// input
			desc := ocispec.Descriptor{
				MediaType: mediaType,
				URLs: []string{
					ts.URL + "/missing",
					ts.URL + "/ok",
				},
			}

			reader, err := fetcher.Fetch(context.Background(), desc)
			require.NoError(t, err, "fetch should succeed from test server")
			defer reader.Close()

			output, err := io.ReadAll(reader)
			assert.NoError(t, err, "should have a valid byte buffer")
			assert.Equal(t, expectedBody, string(output))

			assert.Equal(t, requests, len(desc.URLs), "should have tried all URLs until success")
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
	assert.True(t, errors.Is(err, errdefs.ErrNotFound))
}

func TestFetchManifest(t *testing.T) {
	const (
		registry       = "registry"
		repository     = "repository"
		imageManifest  = "image manifest"
		imageDigest    = "sha256:18019fb68413973fcde9ff917d333bbaa228c4aaebba9ad0ca5ffec26e4f3541"
		imageTag       = "tag"
		imageTagDigest = "tag@" + imageDigest
	)

	// Test all supported media types
	for _, mediaType := range supportedImageMediaTypes {
		// Test variants of Object (tag, digest, and combination).
		for _, testObject := range []struct {
			ImageIdentifier types.ImageIdentifier
			Object          string
		}{
			// Tag alone - used on first get image.
			{Object: imageTag, ImageIdentifier: types.ImageIdentifier{ImageTag: aws.String(imageTag)}},
			// Tag and digest assertive fetch
			{Object: imageTagDigest, ImageIdentifier: types.ImageIdentifier{ImageTag: aws.String(imageTag), ImageDigest: aws.String(imageDigest)}},
			// Digest fetch
			{Object: "@" + imageDigest, ImageIdentifier: types.ImageIdentifier{ImageDigest: aws.String(imageDigest)}},
		} {
			fakeClient := &fakeECRClient{}
			fetcher := &ecrFetcher{
				ecrBase: ecrBase{
					client: fakeClient,
					ecrSpec: ECRSpec{
						arn: arn.ARN{
							AccountID: registry,
						},
						Repository: repository,
						Object:     testObject.Object,
					},
				},
			}
			desc := ocispec.Descriptor{
				MediaType: mediaType,
			}
			if testObject.ImageIdentifier.ImageDigest != nil {
				desc.Digest = digest.Digest(aws.ToString(testObject.ImageIdentifier.ImageDigest))
			}

			t.Run(mediaType+"_"+testObject.Object, func(t *testing.T) {
				callCount := 0
				fakeClient.BatchGetImageFn = func(_ context.Context, input *ecr.BatchGetImageInput, _ ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error) {
					callCount++
					assert.Equal(t, registry, aws.ToString(input.RegistryId))
					assert.Equal(t, repository, aws.ToString(input.RepositoryName))

					assert.ElementsMatch(t, []types.ImageIdentifier{testObject.ImageIdentifier}, input.ImageIds)

					// Fetching populated descriptors uses a narrower requested
					// content type.
					requestedTypes := input.AcceptedMediaTypes
					t.Logf("requestedTypes: %q", requestedTypes)
					if testObject.ImageIdentifier.ImageDigest != nil {
						expectedTypes := []string{desc.MediaType}
						t.Logf("expectedTypes: %q", expectedTypes)
						assert.Equal(t, expectedTypes, requestedTypes,
							"mediaType should match the descriptor")
					} else {
						expectedTypes := supportedImageMediaTypes
						t.Logf("expectedTypes: %q", expectedTypes)
						assert.Equal(t, expectedTypes, requestedTypes,
							"mediaType should allow any supported type")
					}

					return &ecr.BatchGetImageOutput{
						Images: []types.Image{{ImageManifest: aws.String(imageManifest)}},
					}, nil
				}

				reader, err := fetcher.Fetch(context.Background(), desc)
				require.NoError(t, err, "fetch")
				defer reader.Close()
				assert.Equal(t, 1, callCount, "BatchGetImage should be called once")
				manifest, err := io.ReadAll(reader)
				require.NoError(t, err, "reading manifest")
				assert.Equal(t, imageManifest, string(manifest))
			})
		}
	}
}

func TestFetchManifestAPIError(t *testing.T) {
	ref := "ecr.aws/arn:aws:ecr:fake:123456789012:repository/foo/bar:latest"
	mediaType := ocispec.MediaTypeImageManifest

	fakeClient := &fakeECRClient{
		BatchGetImageFn: func(context.Context, *ecr.BatchGetImageInput, ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error) {
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
		BatchGetImageFn: func(context.Context, *ecr.BatchGetImageInput, ...func(*ecr.Options)) (*ecr.BatchGetImageOutput, error) {
			return &ecr.BatchGetImageOutput{
				Failures: []types.ImageFailure{
					{FailureCode: types.ImageFailureCodeImageNotFound},
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
	layerDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{}
	fetcher := &ecrFetcher{
		ecrBase: ecrBase{
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
		ocispec.MediaTypeImageLayerZstd,
		ocispec.MediaTypeImageLayer,
		ocispec.MediaTypeImageConfig,
	} {
		t.Run(mediaType, func(t *testing.T) {
			callCount := 0
			fakeClient.GetDownloadUrlForLayerFn = func(_ context.Context, input *ecr.GetDownloadUrlForLayerInput, _ ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error) {
				callCount++
				assert.Equal(t, registry, aws.ToString(input.RegistryId))
				assert.Equal(t, repository, aws.ToString(input.RepositoryName))
				assert.Equal(t, layerDigest, aws.ToString(input.LayerDigest))
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
			body, err := io.ReadAll(reader)
			assert.NoError(t, err, "reading body")
			assert.Equal(t, expectedBody, string(body))
		})
	}
}

func TestFetchLayerAPIError(t *testing.T) {
	fakeClient := &fakeECRClient{
		GetDownloadUrlForLayerFn: func(context.Context, *ecr.GetDownloadUrlForLayerInput, ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error) {
			return nil, errors.New("expected")
		},
	}
	fetcher := &ecrFetcher{
		ecrBase: ecrBase{
			client: fakeClient,
		},
	}
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
	}
	_, err := fetcher.Fetch(context.Background(), desc)
	assert.Error(t, err)
}

func TestFetchLayerHtcat(t *testing.T) {
	registry := "registry"
	repository := "repository"
	layerDigest := testdata.InsignificantDigest.String()
	fakeClient := &fakeECRClient{}
	fetcher := &ecrFetcher{
		ecrBase: ecrBase{
			client: fakeClient,
			ecrSpec: ECRSpec{
				arn: arn.ARN{
					AccountID: registry,
				},
				Repository: repository,
			},
		},
		parallelism: 2,
	}
	// need >1mb of content for htcat to do parallel requests
	const (
		kB = 1024 * 1
		mB = 1024 * kB
	)
	expectedBody := make([]byte, 30*mB)
	_, err := rand.Read(expectedBody)
	assert.NoError(t, err)
	handlerCallCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		http.ServeContent(w, r, "", time.Now(), bytes.NewReader(expectedBody))
	}))
	defer ts.Close()

	downloadURLCallCount := 0
	fakeClient.GetDownloadUrlForLayerFn = func(_ context.Context, input *ecr.GetDownloadUrlForLayerInput, _ ...func(*ecr.Options)) (*ecr.GetDownloadUrlForLayerOutput, error) {
		downloadURLCallCount++
		assert.Equal(t, registry, aws.ToString(input.RegistryId))
		assert.Equal(t, repository, aws.ToString(input.RepositoryName))
		assert.Equal(t, layerDigest, aws.ToString(input.LayerDigest))
		return &ecr.GetDownloadUrlForLayerOutput{DownloadUrl: aws.String(ts.URL)}, nil
	}
	desc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Layer,
		Digest:    digest.Digest(layerDigest),
	}
	reader, err := fetcher.Fetch(context.Background(), desc)
	assert.NoError(t, err, "fetch")
	defer reader.Close()
	assert.Equal(t, 1, downloadURLCallCount, "GetDownloadURLForLayer should be called once")
	body, err := io.ReadAll(reader)
	assert.NoError(t, err, "reading body")
	assert.Equal(t, expectedBody, body)
	assert.True(t, handlerCallCount > 1, "ServeContent should be called more than once: %d", handlerCallCount)
}
