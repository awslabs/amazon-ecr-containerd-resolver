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
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/stream"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type layerWriter struct {
	ctx      context.Context
	base     *ecrBase
	desc     ocispec.Descriptor
	buf      io.WriteCloser
	tracker  docker.StatusTracker
	ref      string
	uploadID string
	err      chan error
}

var _ content.Writer = (*layerWriter)(nil)

const (
	layerQueueSize = 5
)

func newLayerWriter(base *ecrBase, tracker docker.StatusTracker, ref string, desc ocispec.Descriptor) (content.Writer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("desc", desc))
	reader, writer := io.Pipe()
	lw := &layerWriter{
		ctx:     ctx,
		base:    base,
		desc:    desc,
		buf:     writer,
		tracker: tracker,
		ref:     ref,
		err:     make(chan error),
	}

	// call InitiateLayerUpload and get upload ID
	initiateLayerUploadInput := &ecr.InitiateLayerUploadInput{
		RegistryId:     aws.String(base.ecrSpec.Registry()),
		RepositoryName: aws.String(base.ecrSpec.Repository),
	}
	initiateLayerUploadOutput, err := base.client.InitiateLayerUpload(ctx, initiateLayerUploadInput)
	if err != nil {
		cancel()
		return nil, err
	}
	lw.uploadID = aws.ToString(initiateLayerUploadOutput.UploadId)
	partSize := aws.ToInt64(initiateLayerUploadOutput.PartSize)
	log.G(ctx).
		WithField("digest", desc.Digest.String()).
		WithField("uploadID", lw.uploadID).
		WithField("partSize", partSize).
		Debug("ecr.blob.init")

	go func() {
		defer cancel()
		defer close(lw.err)
		_, err := stream.ChunkedProcessor(reader, partSize, layerQueueSize,
			func(layerChunk *stream.Chunk) error {
				begin := layerChunk.BytesBegin
				end := layerChunk.BytesEnd
				bytesRead := end - begin
				log.G(ctx).
					WithField("digest", desc.Digest.String()).
					WithField("part", layerChunk.Part).
					WithField("begin", begin).
					WithField("end", end).
					WithField("bytes", bytesRead).
					Debug("ecr.layer.callback")

				uploadLayerPartInput := &ecr.UploadLayerPartInput{
					RegistryId:     aws.String(base.ecrSpec.Registry()),
					RepositoryName: aws.String(base.ecrSpec.Repository),
					UploadId:       aws.String(lw.uploadID),
					PartFirstByte:  aws.Int64(begin),
					PartLastByte:   aws.Int64(end),
					LayerPartBlob:  layerChunk.Bytes,
				}

				_, err := base.client.UploadLayerPart(ctx, uploadLayerPartInput)
				log.G(ctx).
					WithField("digest", desc.Digest.String()).
					WithField("part", layerChunk.Part).
					WithField("begin", begin).
					WithField("end", end).
					WithField("bytes", bytesRead).
					Debug("ecr.layer.callback end")
				if err == nil {
					var status docker.Status
					status, err = lw.tracker.GetStatus(lw.ref)
					if err == nil {
						status.Offset += int64(bytesRead) + 1
						status.UpdatedAt = time.Now()
						lw.tracker.SetStatus(lw.ref, status)
					}
				}
				return err
			})
		if err != nil {
			lw.err <- err
		}
		log.G(ctx).WithField("digest", desc.Digest.String()).Debug("ecr.layer upload done")
	}()
	return lw, nil
}

func (lw *layerWriter) Write(b []byte) (int, error) {
	log.G(lw.ctx).WithField("len(b)", len(b)).Debug("ecr.layer.write")
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
	log.G(lw.ctx).WithField("size", size).WithField("expected", expected).Debug("ecr.layer.commit")
	lw.buf.Close()
	select {
	case err := <-lw.err:
		if err != nil {
			log.G(lw.ctx).
				WithError(err).
				WithField("expected", expected).
				Error("ecr.layer.commit: error while uploading parts")
			return err
		}
	case <-lw.ctx.Done():
	}

	completeLayerUploadInput := &ecr.CompleteLayerUploadInput{
		RegistryId:     aws.String(lw.base.ecrSpec.Registry()),
		RepositoryName: aws.String(lw.base.ecrSpec.Repository),
		UploadId:       aws.String(lw.uploadID),
		LayerDigests:   []string{expected.String()},
	}

	completeLayerUploadOutput, err := lw.base.client.CompleteLayerUpload(ctx, completeLayerUploadInput)
	if err != nil {
		// If the layer that is being uploaded already exists then return successfully instead of failing. Unfortunately
		// in this case we do not get the digest back from ECR, but if the client-provided digest starts with a
		// "sha256:" then the ECR has validated that the digest provided matches ours. If the expected digest uses a
		// different algorithm we have to fail as we do not know the digest ECR calculated and the expected digest
		// has not been validated.
		var alreadyExists *types.LayerAlreadyExistsException
		if errors.As(err, &alreadyExists) && strings.HasPrefix(expected.String(), "sha256:") {
			log.G(lw.ctx).Debug("ecr.layer.commit: layer already exists")
			return nil
		}
		return err
	}
	actualDigest := aws.ToString(completeLayerUploadOutput.LayerDigest)
	if actualDigest != expected.String() {
		return errors.New("ecr: failed to validate uploaded digest")
	}
	log.G(ctx).
		WithField("expected", expected).
		WithField("actual", actualDigest).
		Debug("ecr.layer.commit: complete")
	return nil
}

func (lw *layerWriter) Status() (content.Status, error) {
	log.G(lw.ctx).Debug("ecr.layer.status")

	return content.Status{
		Ref: lw.desc.Digest.String(),
	}, nil
}

func (lw *layerWriter) Truncate(size int64) error {
	log.G(lw.ctx).WithField("size", size).Debug("ecr.layer.truncate")

	return errors.New("ecr.layer.truncate: not implemented")
}
