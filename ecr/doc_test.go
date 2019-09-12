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

package ecr_test

import (
	"context"
	"fmt"

	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
)

func ExampleNewResolver_pull() {
	client, _ := containerd.New("/run/containerd/containerd.sock")
	resolver, _ := ecr.NewResolver()
	img, _ := client.Pull(
		namespaces.NamespaceFromEnv(context.TODO()),
		"ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
		containerd.WithResolver(resolver),
		containerd.WithPullUnpack,
		containerd.WithSchema1Conversion)
	fmt.Println(img.Name())
}

func ExampleNewResolver_push() {
	client, _ := containerd.New("/run/containerd/containerd.sock")
	ctx := namespaces.NamespaceFromEnv(context.TODO())

	img, _ := client.ImageService().Get(
		ctx,
		"docker.io/library/busybox:latest")
	resolver, _ := ecr.NewResolver()
	_ = client.Push(
		ctx,
		"ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
		img.Target,
		containerd.WithResolver(resolver))
}
