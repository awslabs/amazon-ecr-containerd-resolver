/*
 * Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context/ctxhttp"
)

type ecrFetcher struct {
	client  ecriface.ECRAPI
	ecrSpec ECRSpec
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func (f *ecrFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	fmt.Printf("fetch: desc=%v\n", desc)
	// need to do different things based on the media type
	switch desc.MediaType {
	case
		ocispec.MediaTypeImageManifest,
		images.MediaTypeDockerSchema2Manifest:
		return f.fetchManifest(ctx, desc)
	case
		images.MediaTypeDockerSchema2Layer,
		images.MediaTypeDockerSchema2LayerGzip,
		images.MediaTypeDockerSchema2Config,
		ocispec.MediaTypeImageLayerGzip,
		ocispec.MediaTypeImageLayer,
		ocispec.MediaTypeImageConfig:
		return f.fetchLayer(ctx, desc)
	default:
		fmt.Printf("fetch: desc=%v mediatype=%s\n", desc, desc.MediaType)
		return nil, unimplemented
	}
	return nil, unimplemented
}

func (f *ecrFetcher) fetchManifest(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	fmt.Printf("fetchManifest: desc%v\n", desc)
	batchGetImageInput := &ecr.BatchGetImageInput{
		RegistryId:         aws.String(f.ecrSpec.Registry),
		RepositoryName:     aws.String(f.ecrSpec.Repository),
		ImageIds:           []*ecr.ImageIdentifier{f.ecrSpec.ImageID()},
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
		fmt.Println("what")
		return nil, reference.ErrInvalid
	}
	ecrImage = batchGetImageOutput.Images[0]
	return nopCloser{bytes.NewReader([]byte(aws.StringValue(ecrImage.ImageManifest)))}, nil
}

func (f *ecrFetcher) fetchLayer(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	fmt.Printf("fetchLayer: desc=%v\n", desc)
	getDownloadUrlForLayerInput := &ecr.GetDownloadUrlForLayerInput{
		RegistryId:     aws.String(f.ecrSpec.Registry),
		RepositoryName: aws.String(f.ecrSpec.Repository),
		LayerDigest:    aws.String(desc.Digest.String()),
	}
	output, err := f.client.GetDownloadUrlForLayer(getDownloadUrlForLayerInput)
	if err != nil {
		return nil, err
	}
	fmt.Printf("fetchLayer: url=%s\n", aws.StringValue(output.DownloadUrl))

	downloadURL := aws.StringValue(output.DownloadUrl)
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	fmt.Printf("fetch: GET to %s\n", downloadURL)

	req.Header.Set("Accept", strings.Join([]string{desc.MediaType, `*`}, ", "))
	resp, err := f.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		//		if resp.StatusCode == http.StatusNotFound {
		//			continue // try one of the other urls.
		//		}
		resp.Body.Close()
		return nil, errors.Errorf("unexpected status code %v: %v", downloadURL, resp.Status)
	}
	fmt.Println("fetch: returning body")
	return resp.Body, nil
}

func (f *ecrFetcher) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	client := http.DefaultClient // TODO
	resp, err := ctxhttp.Do(ctx, client, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do request")
	}
	return resp, nil
}
