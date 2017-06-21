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
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/reference"
	"github.com/pkg/errors"
)

const (
	refPrefix           = "ecr.aws/"
	arnDelimiter        = ":"
	arnSections         = 6
	arnPrefix           = "arn:"
	repositoryDelimiter = "/"

	// zero-indexed
	sectionPartition  = 1
	sectionRegion     = 3
	sectionRegistry   = 4
	sectionRepository = 5
)

var (
	invalidARN = errors.New("ref: invalid ARN")
	splitRe    = regexp.MustCompile(`[:@]`)
)

type ECRSpec struct {
	Partition  string
	Region     string
	Registry   string
	Repository string
	Object     string
}

// ParseRef parses an ECR reference into its constituent parts
func ParseRef(ref string) (ECRSpec, error) {
	if !strings.HasPrefix(ref, refPrefix) {
		return ECRSpec{}, invalidARN
	}
	stripped := ref[len(refPrefix):]
	spec, err := ParseARN(stripped)
	if err != nil {
		return ECRSpec{}, err
	}

	if delimiterIndex := splitRe.FindStringIndex(spec.Repository); delimiterIndex != nil {
		// This allows us to retain the @ to signify digests or shortend digests in
		// the object.
		spec.Object = spec.Repository[delimiterIndex[0]:]
		if spec.Object[:1] == ":" {
			spec.Object = spec.Object[1:]
		}
		spec.Repository = spec.Repository[:delimiterIndex[0]]
	}
	return spec, nil
}

// ParseARN parses an ECR ARN into its constituent parts
// An example ARN is arn:aws:ecr:us-west-2:123456789012:repository/foo/bar
func ParseARN(arn string) (ECRSpec, error) {
	if !strings.HasPrefix(arn, arnPrefix) {
		return ECRSpec{}, invalidARN
	}
	sections := strings.SplitN(arn, arnDelimiter, arnSections)
	if len(sections) != arnSections {
		return ECRSpec{}, invalidARN
	}
	repositorySections := strings.SplitN(sections[sectionRepository], repositoryDelimiter, 2)
	if len(repositorySections) != 2 {
		return ECRSpec{}, invalidARN
	}
	return ECRSpec{
		Partition:  sections[sectionPartition],
		Region:     sections[sectionRegion],
		Registry:   sections[sectionRegistry],
		Repository: repositorySections[1],
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

// ARN returns the canonical representation of the ARN
func (spec ECRSpec) ARN() string {
	return arnPrefix + spec.Partition + arnDelimiter + "ecr" + arnDelimiter + spec.Region + arnDelimiter + spec.Registry + arnDelimiter + "repository/" + spec.Repository
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
