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
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/samuelkarp/amazon-ecr-containerd-resolver/ecr/stream"
)

type layerWriter struct {
	ctx      context.Context
	base     *ecrBase
	desc     ocispec.Descriptor
	buf      io.WriteCloser
	uploadID string
	err      chan error
}

var _ content.Writer = (*layerWriter)(nil)

const (
	layerQueueSize = 5
)

func newLayerWriter(base *ecrBase, desc ocispec.Descriptor) (content.Writer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	lw := &layerWriter{
		ctx:  ctx,
		base: base,
		desc: desc,
		buf:  writer,
		err:  make(chan error),
	}

	// call InitiateLayerUpload and get upload ID
	initiateLayerUploadInput := &ecr.InitiateLayerUploadInput{
		RegistryId:     aws.String(base.ecrSpec.Registry()),
		RepositoryName: aws.String(base.ecrSpec.Repository),
	}
	initiateLayerUploadOutput, err := base.client.InitiateLayerUpload(initiateLayerUploadInput)
	if err != nil {
		return nil, err
	}
	lw.uploadID = aws.StringValue(initiateLayerUploadOutput.UploadId)
	partSize := aws.Int64Value(initiateLayerUploadOutput.PartSize)
	fmt.Printf("lw.init digest=%s uuid=%s partSize=%d\n", desc.Digest.String(), lw.uploadID, partSize)

	go func() {
		defer cancel()
		defer close(lw.err)
		_, err := stream.ChunkedProcessor(reader, partSize, layerQueueSize,
			func(layerChunk *stream.Chunk) error {
				fmt.Printf("lw.callback: digest=%s part=%d\n", desc.Digest.String(), layerChunk.Part)
				begin := layerChunk.BytesBegin
				end := layerChunk.BytesEnd
				bytesRead := end - begin

				uploadLayerPartInput := &ecr.UploadLayerPartInput{
					RegistryId:     aws.String(base.ecrSpec.Registry()),
					RepositoryName: aws.String(base.ecrSpec.Repository),
					UploadId:       aws.String(lw.uploadID),
					PartFirstByte:  aws.Int64(begin),
					PartLastByte:   aws.Int64(end),
					LayerPartBlob:  layerChunk.Bytes,
				}

				_, err := base.client.UploadLayerPart(uploadLayerPartInput)
				fmt.Printf("lw.callback: done digest=%s part=%d bytesRead=%d begin=%d end=%d\n",
					desc.Digest.String(), layerChunk.Part, bytesRead, begin, end)
				return err
			})
		if err != nil {
			lw.err <- err
		}
		fmt.Printf("lw.chunkedReader: done digest=%s\n", desc.Digest.String())
	}()
	return lw, nil
}

func (lw *layerWriter) Write(b []byte) (int, error) {
	fmt.Printf("lw.Write: len(b)=%d\n", len(b))
	select {
	case err := <-lw.err:
		return 0, err
	case <-lw.ctx.Done():
		return 0, errors.New("lw.Write: closed")
	default:
	}
	return lw.buf.Write(b)
}

func (lw *layerWriter) Close() error {
	return errors.New("lw.Close: not implemented")
}

func (lw *layerWriter) Digest() digest.Digest {
	return lw.desc.Digest
}

func (lw *layerWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	fmt.Printf("lw.Commit: size=%d expected=%s\n", size, expected)
	lw.buf.Close()
	select {
	case err := <-lw.err:
		if err != nil {
			fmt.Printf("lw.Commit: expected=%s err=%v\n", expected, err)
			return err
		}
	case <-lw.ctx.Done():
	}

	completeLayerUploadInput := &ecr.CompleteLayerUploadInput{
		RegistryId:     aws.String(lw.base.ecrSpec.Registry()),
		RepositoryName: aws.String(lw.base.ecrSpec.Repository),
		UploadId:       aws.String(lw.uploadID),
		LayerDigests:   []*string{aws.String(expected.String())},
	}

	completeLayerUploadOutput, err := lw.base.client.CompleteLayerUpload(completeLayerUploadInput)
	actualDigest := aws.StringValue(completeLayerUploadOutput.LayerDigest)
	if err != nil {
		// If the layer that is being uploaded already exists then return successfully instead of failing. Unfortunately
		// in this case we do not get the digest back from ECR, but if the client-provided digest starts with a
		// "sha256:" then the ECR has validated that the digest provided matches ours. If the expected digest uses a
		// different algorithm we have to fail as we do not know the digest ECR calculated and the expected digest
		// has not been validated.
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "LayerAlreadyExistsException" && strings.HasPrefix(expected.String(), "sha256:") {
			fmt.Println("ecr: layer already exists")
			return nil
		} else {
			return err
		}
	}
	if actualDigest != expected.String() {
		return errors.New("ecr: failed to validate uploaded digest")
	}
	fmt.Printf("lw.Commit: actual=%s expected=%s\n", actualDigest, expected)
	return nil
}

func (lw *layerWriter) Status() (content.Status, error) {
	//fmt.Println("lw.Status")
	return content.Status{
		Ref: lw.desc.Digest.String(),
	}, nil
}

func (lw *layerWriter) Truncate(size int64) error {
	//fmt.Printf("lw.Truncate: size=%d\n", size)
	return errors.New("lw.Truncate: not implemented")
}
