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
package ecr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseARN(t *testing.T) {
	cases := []struct {
		arn  string
		spec ECRSpec
		err  error
	}{
		{
			arn: "invalid",
			err: invalidARN,
		},
		{
			arn: "arn:nope",
			err: invalidARN,
		},
		{
			arn: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				Partition:  "aws",
				Region:     "us-west-2",
				Registry:   "123456789012",
				Repository: "foo/bar",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.arn, func(t *testing.T) {
			spec, err := ParseARN(tc.arn)
			assert.Equal(t, tc.spec, spec)
			if tc.err == nil {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.err, err)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	cases := []struct {
		ref  string
		spec ECRSpec
		err  error
	}{
		{
			ref: "invalid",
			err: invalidARN,
		},
		{
			ref: "ecr.aws/arn:nope",
			err: invalidARN,
		},
		{
			ref: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			err: invalidARN,
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				Partition:  "aws",
				Region:     "us-west-2",
				Registry:   "123456789012",
				Repository: "foo/bar",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar:latest",
			spec: ECRSpec{
				Partition:  "aws",
				Region:     "us-west-2",
				Registry:   "123456789012",
				Repository: "foo/bar",
				Object:     "latest",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar:latest@sha256:digest",
			spec: ECRSpec{
				Partition:  "aws",
				Region:     "us-west-2",
				Registry:   "123456789012",
				Repository: "foo/bar",
				Object:     "latest@sha256:digest",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar@sha256:digest",
			spec: ECRSpec{
				Partition:  "aws",
				Region:     "us-west-2",
				Registry:   "123456789012",
				Repository: "foo/bar",
				Object:     "@sha256:digest",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			spec, err := ParseRef(tc.ref)
			assert.Equal(t, tc.spec, spec)
			if tc.err == nil {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.err, err)
			}
		})
	}
}
