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

package main

import (
	"context"
	"os"

	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func main() {
	ctx := namespaces.NamespaceFromEnv(context.Background())

	if len(os.Args) < 2 {
		log.G(ctx).Fatal("Must provide image to pull as argument")
	}
	ref := os.Args[1]

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to connect to containerd")
	}
	defer client.Close()

	ongoing := newJobs(ref)
	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})
	go func() {
		showProgress(pctx, ongoing, client.ContentStore(), os.Stdout)
		close(progress)
	}()

	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.add(desc)
		}
		return nil, nil
	})

	resolver, err := ecr.NewResolver()
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to create resolver")
	}

	log.G(ctx).WithField("ref", ref).Info("Pulling from Amazon ECR")
	img, err := client.Pull(ctx, ref,
		containerd.WithResolver(resolver),
		containerd.WithImageHandler(h),
		containerd.WithSchema1Conversion)
	stopProgress()
	if err != nil {
		log.G(ctx).WithError(err).WithField("ref", ref).Fatal("Failed to pull")
	}
	<-progress
	log.G(ctx).WithField("img", img.Name()).Info("Pulled successfully!")

	log.G(ctx).WithField("img", img.Name()).Info("unpacking...")
	err = img.Unpack(ctx, containerd.DefaultSnapshotter)
	if err != nil {
		log.G(ctx).WithError(err).WithField("img", img.Name).Fatal("Failed to unpack")
	}
}
