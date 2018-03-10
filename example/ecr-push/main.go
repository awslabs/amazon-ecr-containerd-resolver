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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/samuelkarp/amazon-ecr-containerd-resolver/ecr"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx := namespaces.NamespaceFromEnv(context.Background())
	logrus.SetLevel(logrus.DebugLevel)

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

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to connect to containerd")
	}
	defer client.Close()

	awsSession, err := session.NewSession()
	if err != nil {
		log.G(ctx).WithError(err).Fatal("Failed to create AWS session")
	}

	img, err := client.ImageService().Get(ctx, local)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	desc := img.Target

	log.G(ctx).WithField("local", local).WithField("ref", ref).Info("Pushing to Amazon ECR")
	err = client.Push(ctx, ref, desc,
		containerd.WithResolver(ecr.NewResolver(awsSession)))
	if err != nil {
		log.G(ctx).WithError(err).WithField("ref", ref).Fatal("Failed to push")

	}
	log.G(ctx).WithField("ref", ref).Info("Pushed successfully!")
}
