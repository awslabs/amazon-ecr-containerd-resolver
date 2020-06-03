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

// Package ecr provides implementations of the containerd Resolver, Fetcher, and
// Pusher interfaces that can use the Amazon ECR API to push and pull images.
//
// References
//
// containerd specifies images with a reference, or a "ref".  References are
// different from Docker image names, as references encode an identifier, but
// not a retrieval mechanism. refs start with a DNS-style namespace that can be
// used to select separate Resolvers to use.
//
// The canonical ref format used by this package is ecr.aws/ followed by the ARN
// of the repository and a label and/or a digest.  Valid references are of the
// form "ecr.aws/arn:aws:ecr:<region>:<account>:repository/<name>:<tag>".
//
// License
//
// This package is licensed under the Apache 2.0 license.
package ecr
