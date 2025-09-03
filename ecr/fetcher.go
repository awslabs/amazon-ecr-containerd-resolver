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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	httputil "github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/util/http"
	ociutil "github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/util/oci"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/htcat/htcat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context/ctxhttp"
)

// ecrFetcher implements the containerd remotes.Fetcher interface and can be
// used to pull images from Amazon ECR.
type ecrFetcher struct {
	ecrBase
	parallelism int
	httpClient  *http.Client
}

var _ remotes.Fetcher = (*ecrFetcher)(nil)

func (f *ecrFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("desc", ociutil.RedactDescriptor(desc)))
	log.G(ctx).Debug("ecr.fetch")

	// need to do different things based on the media type
	switch desc.MediaType {
	case
		images.MediaTypeDockerSchema1Manifest,
		images.MediaTypeDockerSchema2Manifest,
		images.MediaTypeDockerSchema2ManifestList,
		ocispec.MediaTypeImageIndex,
		ocispec.MediaTypeImageManifest:
		return f.fetchManifest(ctx, desc)
	case
		images.MediaTypeDockerSchema2Layer,
		images.MediaTypeDockerSchema2LayerGzip,
		images.MediaTypeDockerSchema2Config,
		ocispec.MediaTypeImageLayerGzip,
		ocispec.MediaTypeImageLayerZstd,
		ocispec.MediaTypeImageLayer,
		ocispec.MediaTypeImageConfig:
		return f.fetchLayer(ctx, desc)
	case
		images.MediaTypeDockerSchema2LayerForeign,
		images.MediaTypeDockerSchema2LayerForeignGzip:
		return f.fetchForeignLayer(ctx, desc)
	default:
		log.G(ctx).
			WithField("media type", desc.MediaType).
			Error("ecr.fetcher: unimplemented media type")
		return nil, unimplemented
	}
}

func (f *ecrFetcher) fetchManifest(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	var (
		image *types.Image
		err   error
	)
	// A digest is required to fetch by digest alone. When a digest is not
	// provided the fetch is based on the parsed ECR resource - specifying both
	// a digest and tag in the request if possible.
	if desc.Digest == "" {
		log.G(ctx).Debug("ecr.fetcher.manifest: fetch image by tag")
		image, err = f.getImage(ctx)
	} else {
		log.G(ctx).Debug("ecr.fetcher.manifest: fetch image by digest")
		image, err = f.getImageByDescriptor(ctx, desc)
	}
	if err != nil {
		return nil, err
	}
	if image == nil {
		return nil, errors.New("fetchManifest: nil image")
	}

	return io.NopCloser(bytes.NewReader([]byte(aws.ToString(image.ImageManifest)))), nil
}

func (f *ecrFetcher) fetchLayer(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	log.G(ctx).Debug("ecr.fetcher.layer")
	getDownloadUrlForLayerInput := &ecr.GetDownloadUrlForLayerInput{
		RegistryId:     aws.String(f.ecrSpec.Registry()),
		RepositoryName: aws.String(f.ecrSpec.Repository),
		LayerDigest:    aws.String(desc.Digest.String()),
	}
	output, err := f.client.GetDownloadUrlForLayer(ctx, getDownloadUrlForLayerInput)
	if err != nil {
		return nil, err
	}

	downloadURL := aws.ToString(output.DownloadUrl)
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("url", httputil.RedactHTTPQueryValuesFromURL(downloadURL)))
	if f.parallelism > 0 {
		return f.fetchLayerHtcat(ctx, desc, downloadURL)
	}
	return f.fetchLayerURL(ctx, desc, downloadURL)
}

func (f *ecrFetcher) fetchForeignLayer(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	log.G(ctx).Debug("ecr.fetcher.layer.foreign")
	if len(desc.URLs) < 1 {
		log.G(ctx).Error("cannot pull foreign layer without URL")
	}
	var err error
	for _, layerURL := range desc.URLs {
		redactedDownloadURL := httputil.RedactHTTPQueryValuesFromURL(layerURL)
		log.G(ctx).WithField("url", redactedDownloadURL).Debug("ecr.fetcher.layer.foreign: fetching from URL")
		var rdc io.ReadCloser
		rdc, err = f.fetchLayerURL(ctx, desc, layerURL)
		if err == nil {
			return rdc, nil
		}
		log.G(ctx).WithField("url", redactedDownloadURL).WithError(err).Warn("ecr.fetcher.layer.foreign: unable to fetch from URL")
	}
	return nil, err
}

func (f *ecrFetcher) fetchLayerURL(ctx context.Context, desc ocispec.Descriptor, downloadURL string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		log.G(ctx).
			WithError(err).
			Error("ecr.fetcher.layer.url: failed to create HTTP request")
		return nil, err
	}
	log.G(ctx).Debug("ecr.fetcher.layer.url")

	req.Header.Set("Accept", strings.Join([]string{desc.MediaType, `*`}, ", "))
	resp, err := f.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		resp.Body.Close()
		redactedDownloadURL := httputil.RedactHTTPQueryValuesFromURL(downloadURL)
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("content at %v not found: %w", redactedDownloadURL, errdefs.ErrNotFound)
		}
		return nil, fmt.Errorf("ecr.fetcher.layer.url: unexpected status code %v: %v", redactedDownloadURL, resp.Status)
	}
	log.G(ctx).Debug("ecr.fetcher.layer.url: returning body")
	return resp.Body, nil
}

func (f *ecrFetcher) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	client := f.httpClient
	resp, err := ctxhttp.Do(ctx, client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", httputil.RedactHTTPQueryValuesFromURLError(err))
	}
	return resp, nil
}

func (f *ecrFetcher) fetchLayerHtcat(ctx context.Context, desc ocispec.Descriptor, downloadURL string) (io.ReadCloser, error) {
	log.G(ctx).Debug("ecr.fetcher.layer.htcat")
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		log.G(ctx).
			WithError(err).
			Error("ecr.fetcher.layer.htcat: failed to parse URL")
		return nil, err
	}
	hc := f.httpClient
	if hc == nil {
		hc = http.DefaultClient
	}
	htc := htcat.New(hc, parsedURL, f.parallelism)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_, err := htc.WriteTo(pw)
		if err != nil {
			log.G(ctx).
				WithError(httputil.RedactHTTPQueryValuesFromURLError(err)).
				Error("ecr.fetcher.layer.htcat: failed to download layer")
		}
	}()
	return pr, nil
}
