/*
 * Copyright 2017-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	errLayerNotFound = errors.New("ecr: layer not found")
)

// ecrPusher implements the containerd remotes.Pusher interface and can be used
// to push images to Amazon ECR.
type ecrPusher struct {
	ecrBase
}

var _ remotes.Pusher = (*ecrPusher)(nil)

func (p ecrPusher) Push(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("push: desc=%v\n", desc)

	switch desc.MediaType {
	case
		ocispec.MediaTypeImageManifest,
		images.MediaTypeDockerSchema2Manifest,
		images.MediaTypeDockerSchema1Manifest:
		return p.pushManifest(ctx, desc)
	default:
		return p.pushBlob(ctx, desc)
	}

	return nil, unimplemented
}

func (p ecrPusher) pushManifest(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("pushManifest: desc=%v\n", desc)
	exists, err := p.checkManifestExistence(ctx, desc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if exists {
		fmt.Println("exists")
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "content %v on remote", desc.Digest)
	}

	return &manifestWriter{
		base: &p.ecrBase,
	}, nil
}

func (p ecrPusher) checkManifestExistence(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	image, err := p.getManifest(ctx)
	if err != nil {
		if err == errImageNotFound {
			return false, nil
		}
		return false, err
	}
	if image == nil {
		return false, errors.New("checkManifestExistence: unexpected nil image")
	}

	found := desc.Digest.String() == aws.StringValue(image.ImageId.ImageDigest)
	return found, nil
}

func (p ecrPusher) pushBlob(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	fmt.Printf("pushBlob: desc=%v\n", desc)
	exists, err := p.checkBlobExistence(ctx, desc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if exists {
		fmt.Println("exists")
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "content %v on remote", desc.Digest)
	}

	// TODO blob push
	return nil, errors.New("pushBlob: not implemented")
}

func (p ecrPusher) checkBlobExistence(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	fmt.Printf("checkBlobExistence: desc=%v\n", desc)

	batchCheckLayerAvailabilityInput := &ecr.BatchCheckLayerAvailabilityInput{
		RegistryId: aws.String(p.ecrSpec.Registry()),
		RepositoryName: aws.String(p.ecrSpec.Repository),
		LayerDigests: []*string{aws.String(desc.Digest.String())},
	}

	batchCheckLayerAvailabilityOutput, err := p.client.BatchCheckLayerAvailability(batchCheckLayerAvailabilityInput)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	fmt.Println(batchCheckLayerAvailabilityOutput)

	if len(batchCheckLayerAvailabilityOutput.Layers) != 1 {
		if len(batchCheckLayerAvailabilityOutput.Failures) > 0 {
			return false, errLayerNotFound
		}
		return false, reference.ErrInvalid
	}

	return true, nil
}


type manifestWriter struct {
	base *ecrBase
	buf bytes.Buffer
}

var _ content.Writer = (*manifestWriter)(nil)

func (mw *manifestWriter) Write(p []byte) (int, error) {
	fmt.Printf("mw.Write: b=%s\n", string(p))
	return mw.buf.Write(p)
}

func (mw *manifestWriter) Close() error {
	return errors.New("mw.Close: not implemented")
}

func (mw *manifestWriter) Digest() digest.Digest {
	return ""
}

func (mw *manifestWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	fmt.Printf("mw.Commit: size=%d expected=%s\n", size, expected)
	manifest := mw.buf.String()
	fmt.Println(manifest)
	ecrSpec := mw.base.ecrSpec
	tag, _ := ecrSpec.TagDigest()
	putImageInput := &ecr.PutImageInput{
		RegistryId: aws.String(ecrSpec.Registry()),
		RepositoryName: aws.String(ecrSpec.Repository),
		ImageTag: aws.String(tag),
		ImageManifest: aws.String(manifest),
	}
	fmt.Printf("%v\n",putImageInput)

	output, err := mw.base.client.PutImage(putImageInput)
	if err != nil {
		return errors.Wrapf(err, "ecr: failed to put manifest: %s", ecrSpec)
	}

	if output == nil {
		return errors.Errorf("ecr: failed to put manifest, nil output: %s", ecrSpec)
	}
	actual := aws.StringValue(output.Image.ImageId.ImageDigest)
	if actual != expected.String() {
		return errors.Errorf("got digest %s, expected %s", actual, expected)
	}
	return nil
}

func (mw *manifestWriter) Status() (content.Status, error) {
	fmt.Println("mw.Status")
	// TODO implement?
	// need at least ref to be populated for good error messages
	return content.Status{}, nil
}

func (mw *manifestWriter) Truncate(size int64) error {
	fmt.Printf("mw.Truncate: size=%d\n", size)
	return errors.New("mw.Truncate: not implemented")
}