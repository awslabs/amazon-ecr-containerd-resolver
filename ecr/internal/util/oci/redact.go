/*
 * Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package oci

import (
	httputil "github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/util/http"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// RedactDescriptor returns a copy of the provided image descriptor
// with its URLs redacted.
func RedactDescriptor(desc ocispec.Descriptor) ocispec.Descriptor {
	for i, url := range desc.URLs {
		desc.URLs[i] = httputil.RedactHTTPQueryValuesFromURL(url)
	}
	return desc
}
