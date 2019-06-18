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
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/stretchr/testify/assert"
)

func TestRefRepresentations(t *testing.T) {
	cases := []struct {
		ref  string
		arn  string
		spec ECRSpec
		err  error
	}{
		{
			ref: "invalid",
			err: invalidARN,
		},
		{
			ref: "ecr.aws/arn:nope",
			err: errors.New("arn: not enough sections"),
		},
		{
			ref: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			err: invalidARN,
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			arn: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				arn: arn.ARN{
					Partition: "aws",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Service:   "ecr",
					Resource:  "repository/foo/bar",
				},
				Repository: "foo/bar",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar:latest",
			arn: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				arn: arn.ARN{
					Partition: "aws",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Service:   "ecr",
					Resource:  "repository/foo/bar",
				},
				Repository: "foo/bar",
				Object:     "latest",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar:latest@sha256:digest",
			arn: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				arn: arn.ARN{
					Partition: "aws",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Service:   "ecr",
					Resource:  "repository/foo/bar",
				},
				Repository: "foo/bar",
				Object:     "latest@sha256:digest",
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar@sha256:digest",
			arn: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			spec: ECRSpec{
				arn: arn.ARN{
					Partition: "aws",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Service:   "ecr",
					Resource:  "repository/foo/bar",
				},
				Repository: "foo/bar",
				Object:     "@sha256:digest",
			},
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("ParseRef-%s", tc.ref), func(t *testing.T) {
			spec, err := ParseRef(tc.ref)
			assert.Equal(t, tc.spec, spec)
			if tc.err == nil {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.err, err)
			}
		})
		if tc.err != nil {
			continue
		}
		t.Run(fmt.Sprintf("Canonical-%s", tc.ref), func(t *testing.T) {
			assert.Equal(t, tc.ref, tc.spec.Canonical())
		})
		t.Run(fmt.Sprintf("ARN-%s", tc.ref), func(t *testing.T) {
			assert.Equal(t, tc.arn, tc.spec.ARN())
		})
	}
}

func TestImageID(t *testing.T) {
	cases := []struct {
		name    string
		spec    ECRSpec
		imageID *ecr.ImageIdentifier
	}{
		{
			name: "blank",
			spec: ECRSpec{
				Repository: "foo/bar",
			},
			imageID: &ecr.ImageIdentifier{},
		},
		{
			name: "tag",
			spec: ECRSpec{
				Repository: "foo/bar",
				Object:     "latest",
			},
			imageID: &ecr.ImageIdentifier{
				ImageTag: aws.String("latest"),
			},
		},
		{
			name: "digest",
			spec: ECRSpec{
				Repository: "foo/bar",
				Object:     "@sha256:digest",
			},
			imageID: &ecr.ImageIdentifier{
				ImageDigest: aws.String("sha256:digest"),
			},
		},
		{
			name: "tag+digest",
			spec: ECRSpec{
				Repository: "foo/bar",
				Object:     "latest@sha256:digest",
			},
			imageID: &ecr.ImageIdentifier{
				ImageTag:    aws.String("latest"),
				ImageDigest: aws.String("sha256:digest"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.imageID, tc.spec.ImageID())
		})
	}
}
