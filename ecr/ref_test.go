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
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar:latest@" + testdata.ImageDigest.String(),
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
				Object:     "latest@" + testdata.ImageDigest.String(),
			},
		},
		{
			ref: "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/foo/bar@" + testdata.ImageDigest.String(),
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
				Object:     "@" + testdata.ImageDigest.String(),
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
				Object:     "@" + testdata.ImageDigest.String(),
			},
			imageID: &ecr.ImageIdentifier{
				ImageDigest: aws.String(testdata.ImageDigest.String()),
			},
		},
		{
			name: "tag+digest",
			spec: ECRSpec{
				Repository: "foo/bar",
				Object:     "latest@" + testdata.ImageDigest.String(),
			},
			imageID: &ecr.ImageIdentifier{
				ImageTag:    aws.String("latest"),
				ImageDigest: aws.String(testdata.ImageDigest.String()),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.imageID, tc.spec.ImageID())
		})
	}
}

// Test ParseEcrImageNameToRef with a valid ECR image name
func TestParseImageURIValid(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		expected  string
	}{
		{
			"Standard",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/my_image:latest",
			"ecr.aws/arn:aws:ecr:us-west-2:777777777777:repository/my_image:latest",
		},
		{
			"Standard: With additional repository path",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/foo/bar/my_image:latest",
			"ecr.aws/arn:aws:ecr:us-west-2:777777777777:repository/foo/bar/my_image:latest",
		},
		{
			"Standard: Digests",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/my_image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"ecr.aws/arn:aws:ecr:us-west-2:777777777777:repository/my_image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			"Standard: Digests with additional repository path",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/baz/my_image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"ecr.aws/arn:aws:ecr:us-west-2:777777777777:repository/baz/my_image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			"AWS CN partition",
			"777777777777.dkr.ecr.cn-north-1.amazonaws.com.cn/my_image:latest",
			"ecr.aws/arn:aws-cn:ecr:cn-north-1:777777777777:repository/my_image:latest",
		},
		{
			"AWS Gov Cloud West",
			"777777777777.dkr.ecr.us-gov-west-1.amazonaws.com/my_image:latest",
			"ecr.aws/arn:aws-us-gov:ecr:us-gov-west-1:777777777777:repository/my_image:latest",
		},
		{
			"AWS Gov Cloud East",
			"777777777777.dkr.ecr.us-gov-east-1.amazonaws.com/my_image:latest",
			"ecr.aws/arn:aws-us-gov:ecr:us-gov-east-1:777777777777:repository/my_image:latest",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("input: %q", tc.imageName)
			result, err := ParseImageURI(tc.imageName)
			require.NoError(t, err, "failed to convert image name into ref")
			assert.Equal(t, tc.expected, result.Canonical())
		})
	}
}

// Test ParseEcrImageNameToRef with an invalid ECR image name
func TestParseImageURIInvalid(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
	}{
		{
			"empty",
			"",
		},
		{
			"no account",
			"dkr.ecr.us-west-2.amazonaws.com",
		},
		{
			"no region",
			"777777777777.dkr.ecr.amazonaws.com/",
		},
		{
			"not an ecr image",
			"docker.io/library/hello-world",
		},
		{
			"missing repository",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/",
		},
		{
			"missing digest value",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/repo-name@",
		},
		{
			"missing label value",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/repo-name:",
		},
		{
			"missing name and label value",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/:",
		},
		{
			"missing typed digest part",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/repo-name@sha256:",
		},
		{
			"invalid typed digest part",
			"777777777777.dkr.ecr.us-west-2.amazonaws.com/repo-name@sha256:invalid-digest-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("input: %q", tc.imageName)
			_, err := ParseImageURI(tc.imageName)
			assert.Error(t, err)
		})
	}
}
