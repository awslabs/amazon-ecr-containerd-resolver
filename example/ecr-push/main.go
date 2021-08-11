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

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	// Default to no debug logging.
	defaultEnableDebug = 0
)

func main() {
	ctx := namespaces.NamespaceFromEnv(context.Background())
	//logrus.SetLevel(logrus.DebugLevel)

	if len(os.Args) < 2 {
		log.G(ctx).Fatal("Must provide image to push as argument")
	}
	ref := os.Args[1]
	local := ""
	if len(os.Args) > 2 {
		local = os.Args[2]
	} else {
		local = ref
	}

	enableDebug := defaultEnableDebug
	parseEnvInt(ctx, "ECR_PUSH_DEBUG", &enableDebug)
	if enableDebug == 1 {
		log.L.Logger.SetLevel(logrus.TraceLevel)
	}

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to connect to containerd")
	}
	defer client.Close()

	tracker := docker.NewInMemoryTracker()
	resolver, err := ecr.NewResolver(ecr.WithTracker(tracker))
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to create resolver")
	}

	img, err := client.ImageService().Get(ctx, local)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ongoing := newPushJobs(tracker)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		log.G(ctx).WithField("local", local).WithField("ref", ref).Info("Pushing to Amazon ECR")
		desc := img.Target

		jobHandler := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			ongoing.add(remotes.MakeRefKey(ctx, desc))
			return nil, nil
		})

		return client.Push(ctx, ref, desc,
			containerd.WithResolver(resolver),
			containerd.WithImageHandler(jobHandler))
	})
	errs := make(chan error)
	go func() {
		defer close(errs)
		errs <- eg.Wait()
	}()

	err = displayUploadProgress(ctx, ongoing, errs)
	if err != nil {
		log.G(ctx).WithError(err).WithField("ref", ref).Fatal("Failed to push")
	}
	log.G(ctx).WithField("ref", ref).Info("Pushed successfully!")
}

func displayUploadProgress(ctx context.Context, ongoing *pushjobs, errs chan error) error {
	var (
		ticker = time.NewTicker(100 * time.Millisecond)
		fw     = progress.NewWriter(os.Stdout)
		start  = time.Now()
		done   bool
	)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fw.Flush()

			tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)

			display(tw, ongoing.status(), start)
			tw.Flush()

			if done {
				fw.Flush()
				return nil
			}
		case err := <-errs:
			if err != nil {
				return err
			}
			done = true
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}
}

func parseEnvInt(ctx context.Context, varname string, val *int) {
	if varval := os.Getenv(varname); varval != "" {
		parsed, err := strconv.Atoi(varval)
		if err != nil {
			log.G(ctx).WithError(err).Fatalf("Failed to parse %s", varname)
		}
		*val = parsed
	}
}
