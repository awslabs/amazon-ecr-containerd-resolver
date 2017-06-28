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
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/samuelkarp/amazon-ecr-containerd-resolver/ecr"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("must provide image to pull as argument")
		os.Exit(1)
	}
	ref := os.Args[1]

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	awsSession, err := session.NewSession()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx := namespaces.NamespaceFromEnv(context.Background())
	img, err := client.Pull(ctx, ref,
		containerd.WithResolver(ecr.NewResolver(awsSession)),
		containerd.WithPullUnpack,
		containerd.WithSchema1Conversion)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(img)
}
