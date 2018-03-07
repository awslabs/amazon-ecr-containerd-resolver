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
package ecr

import (
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/reference"
	"github.com/pkg/errors"
)

const (
	refPrefix           = "ecr.aws/"
	repositoryDelimiter = "/"
)

var (
	invalidARN = errors.New("ref: invalid ARN")
	splitRe    = regexp.MustCompile(`[:@]`)
)

type ECRSpec struct {
	Repository string
	Object     string
	arn        arn.ARN
}

// ParseRef parses an ECR reference into its constituent parts
func ParseRef(ref string) (ECRSpec, error) {
	if !strings.HasPrefix(ref, refPrefix) {
		return ECRSpec{}, invalidARN
	}
	stripped := ref[len(refPrefix):]
	return parseARN(stripped)
}

func (spec ECRSpec) Partition() string {
	return spec.arn.Partition
}

func (spec ECRSpec) Region() string {
	return spec.arn.Region
}

func (spec ECRSpec) Registry() string {
	return spec.arn.AccountID
}

// parseARN parses an ECR ARN into its constituent parts
// An example ARN is arn:aws:ecr:us-west-2:123456789012:repository/foo/bar
func parseARN(a string) (ECRSpec, error) {
	parsed, err := arn.Parse(a)
	if err != nil {
		return ECRSpec{}, err
	}

	// remove label & digest
	var object string
	if delimiterIndex := splitRe.FindStringIndex(parsed.Resource); delimiterIndex != nil {
		// This allows us to retain the @ to signify digests or shortened digests in
		// the object.
		object = parsed.Resource[delimiterIndex[0]:]
		// trim leading :
		if object[:1] == ":" {
			object = object[1:]
		}
		parsed.Resource = parsed.Resource[:delimiterIndex[0]]
	}

	// strip "repository/" prefix
	repositorySections := strings.SplitN(parsed.Resource, repositoryDelimiter, 2)
	if len(repositorySections) != 2 {
		return ECRSpec{}, invalidARN
	}
	return ECRSpec{
		arn:        parsed,
		Repository: repositorySections[1],
		Object:     object,
	}, nil

}

// Canonical returns the canonical representation
func (spec ECRSpec) Canonical() string {
	object := ""
	if len(spec.Object) != 0 {
		if spec.Object[0] != '@' {
			object = ":"
		}
		object = object + spec.Object
	}
	return refPrefix + spec.ARN() + object
}

// ARN returns the canonical representation of the ECR ARN
func (spec ECRSpec) ARN() string {
	return spec.arn.String()
}

// Spec returns a reference.Spec
func (spec ECRSpec) Spec() reference.Spec {
	return reference.Spec{Locator: "", Object: spec.Object}
}

// ImageID returns an ecr.ImageIdentifier suitable for using in calls to ECR
func (spec ECRSpec) ImageID() *ecr.ImageIdentifier {
	imageID := ecr.ImageIdentifier{}
	tag, dgst := reference.SplitObject(spec.Object)
	if tag != "" {
		imageID.ImageTag = aws.String(tag)
	}
	if dgst != "" {
		imageID.ImageDigest = aws.String(string(dgst))
	}
	return &imageID
}
