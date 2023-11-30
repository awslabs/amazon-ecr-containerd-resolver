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
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestRedactDescriptor(t *testing.T) {
	testCases := []struct {
		Name        string
		Description string
		Descriptor  ocispec.Descriptor
		Assert      func(*testing.T, ocispec.Descriptor)
	}{
		{
			Name:        "RedactDescriptorEmptyURLs",
			Description: "Utility should make a descriptor copy with no URLs",
			Descriptor: ocispec.Descriptor{
				URLs: []string{},
			},
			Assert: func(t *testing.T, actual ocispec.Descriptor) {
				if len(actual.URLs) != 0 {
					t.Fatalf("Expected length of 0, got length %d", len(actual.URLs))
				}
			},
		},
		{
			Name:        "RedactDescriptorURLs",
			Description: "Utility should make a descriptor copy with redacted URLs",
			Descriptor: ocispec.Descriptor{
				URLs: []string{
					"s3.amazon.com/foo/bar?token=12345",
					"s3.amazon.com/foo/baz?username=admin&password=admin",
				},
			},
			Assert: func(t *testing.T, actual ocispec.Descriptor) {
				if len(actual.URLs) != 2 {
					t.Fatalf("Expected length of 2, got length %d", len(actual.URLs))
				}
				const expectedURL1 = "s3.amazon.com/foo/bar?token=redacted"
				if actual.URLs[0] != expectedURL1 {
					t.Fatalf("Expected %s; got %s", expectedURL1, actual.URLs[0])
				}
				const expectedURL2 = "s3.amazon.com/foo/baz?password=redacted&username=redacted"
				if actual.URLs[1] != expectedURL2 {
					t.Fatalf("Expected %s; got %s", expectedURL2, actual.URLs[1])
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			actual := RedactDescriptor(testCase.Descriptor)
			testCase.Assert(t, actual)
		})
	}
}
