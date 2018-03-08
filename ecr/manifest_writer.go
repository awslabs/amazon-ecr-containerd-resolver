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
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type manifestWriter struct {
	base *ecrBase
	desc ocispec.Descriptor
	buf  bytes.Buffer
}

var _ content.Writer = (*manifestWriter)(nil)

func (mw *manifestWriter) Write(b []byte) (int, error) {
	fmt.Printf("mw.Write: len(b)=%d\n", len(b))
	return mw.buf.Write(b)
}

func (mw *manifestWriter) Close() error {
	return errors.New("mw.Close: not implemented")
}

func (mw *manifestWriter) Digest() digest.Digest {
	return mw.desc.Digest
}

func (mw *manifestWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	fmt.Printf("mw.Commit: size=%d expected=%s\n", size, expected)
	manifest := mw.buf.String()
	fmt.Println(manifest)
	ecrSpec := mw.base.ecrSpec
	tag, _ := ecrSpec.TagDigest()
	putImageInput := &ecr.PutImageInput{
		RegistryId:     aws.String(ecrSpec.Registry()),
		RepositoryName: aws.String(ecrSpec.Repository),
		ImageTag:       aws.String(tag),
		ImageManifest:  aws.String(manifest),
	}
	fmt.Printf("%v\n", putImageInput)

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
	return content.Status{
		Ref: mw.Digest().String(),
	}, nil
}

func (mw *manifestWriter) Truncate(size int64) error {
	fmt.Printf("mw.Truncate: size=%d\n", size)
	return errors.New("mw.Truncate: not implemented")
}
