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
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/containerd/containerd/reference"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	refPrefix           = "ecr.aws/"
	repositoryDelimiter = "/"
	invalidImageURI = "ecrspec: Invalid image URI"
)

var (
	invalidARN = errors.New("ref: invalid ARN")
	splitRe    = regexp.MustCompile(`[:@]`)
	// Expecting to match ECR image names of the form:
	// Example 1: 777777777777.dkr.ecr.us-west-2.amazonaws.com/my_image:latest
	// Example 2: 777777777777.dkr.ecr.cn-north-1.amazonaws.com.cn/my_image:latest
	// TODO: Support ECR FIPS endpoints, i.e "ecr-fips" in the URL instead of "ecr"
	ecrRegex = regexp.MustCompile(`(^[a-zA-Z0-9][a-zA-Z0-9-_]*)\.dkr\.ecr\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.amazonaws\.com(\.cn)?.*`)
)

// ECRSpec represents a parsed reference.
//
// Valid references are of the form "ecr.aws/arn:aws:ecr:<region>:<account>:repository/<name>:<tag>".
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

// ParseImageURI takes an ECR image URI and then constructs and returns an ECRSpec struct
func ParseImageURI(input string) (ECRSpec, error) {
	input = strings.TrimPrefix(input, "https://")

	// Matching on account, region
	matches := ecrRegex.FindStringSubmatch(input)
	if len(matches) < 3 {
		return ECRSpec{}, errors.New(invalidImageURI)
	}
	region := matches[2]
	account := matches[1]

	// Get the correct partition given its region
	partition, found := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	if !found {
		return ECRSpec{}, errors.New(invalidImageURI)
	}

	// Need to include the full repository path and the imageID (e.g. /eks/image-name:tag)
	tokens := strings.SplitN(input, "/", 2)
	fullRepoPath := tokens[len(tokens)-1]

	// Build the ECR ARN
	ecrARN := arn.ARN{
		Partition: partition.ID(),
		Service:   "ecr",
		Region:    region,
		AccountID: account,
		Resource:  "repository/" + fullRepoPath,
	}
	var object string
	ecrARN.Resource, object = splitResource(ecrARN.Resource)

	return ECRSpec{
		Repository: fullRepoPath,
		Object:     object,
		arn:        ecrARN,
	}, nil
}

// Partition returns the AWS partition
func (spec ECRSpec) Partition() string {
	return spec.arn.Partition
}

// Region returns the AWS region
func (spec ECRSpec) Region() string {
	return spec.arn.Region
}

// Registry returns the Amazon ECR registry
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
	var object string
	parsed.Resource, object = splitResource(parsed.Resource)

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

// splitResource parses the resource segment of an ECR ARN, returns the tag/digest (object) and returns the
// resource segment with the tag/digest (object) removed.
// An example of an ECR ARN resource is "repository/myimage:mytag" and the object returned is "mytag", resource returned is "repository/myimage"
// Another example is "repository/myimage@sha256:47bfdb88c..." and the object returned is "@sha256:47bfdb88c...", resource returned is "repository/myimage"
func splitResource(resource string) (string, string) {
	// remove label & digest
	var object string
	if delimiterIndex := splitRe.FindStringIndex(resource); delimiterIndex != nil {
		// This allows us to retain the @ to signify digests or shortened digests in
		// the object.
		object = resource[delimiterIndex[0]:]
		// trim leading :
		if object[:1] == ":" {
			object = object[1:]
		}
		resource = resource[:delimiterIndex[0]]
	}
	return resource, object
}

// Canonical returns the canonical representation for the reference
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
	tag, digest := spec.TagDigest()
	if tag != "" {
		imageID.ImageTag = aws.String(tag)
	}
	if digest != "" {
		imageID.ImageDigest = aws.String(string(digest))
	}
	return &imageID
}

// TagDigest returns the tag and/or digest specified by the reference
func (spec ECRSpec) TagDigest() (string, digest.Digest) {
	tag, digest := reference.SplitObject(spec.Object)
	tag = strings.TrimSuffix(tag, "@")
	return tag, digest
}
