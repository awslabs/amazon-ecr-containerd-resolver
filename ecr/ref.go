/*
 * Copyright 2017-2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/awslabs/amazon-ecr-containerd-resolver/ecr/internal/awsrulesfn"
	"github.com/containerd/containerd/reference"
	"github.com/opencontainers/go-digest"
)

const (
	refPrefix        = "ecr.aws/"
	repositoryPrefix = "repository/"
	arnServiceID     = "ecr"
)

var (
	invalidARN = errors.New("ref: invalid ARN")
	// Expecting to match ECR image names of the form:
	// Example 1: 777777777777.dkr.ecr.us-west-2.amazonaws.com/my_image:latest
	// Example 2: 777777777777.dkr.ecr.cn-north-1.amazonaws.com.cn/my_image:latest
	// TODO: Support ECR FIPS endpoints, i.e "ecr-fips" in the URL instead of "ecr"
	ecrRegex           = regexp.MustCompile(`(^[a-zA-Z0-9][a-zA-Z0-9-_]*)\.dkr\.ecr\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.amazonaws\.com(\.cn)?/.*`)
	errInvalidImageURI = errors.New("ecrspec: invalid image URI")
)

// ECRSpec represents a parsed reference.
//
// Valid references are of the form "ecr.aws/arn:aws:ecr:<region>:<account>:repository/<name>:<tag>".
type ECRSpec struct {
	// Repository name for this reference.
	Repository string
	// Object is the image reference's object descriptor. This may be a label or
	// a digest specifier.
	Object string
	// arn holds the canonical AWS resource name for this reference.
	arn arn.ARN
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
		return ECRSpec{}, errInvalidImageURI
	}
	account := matches[1]
	region := matches[2]

	// Get the correct partition given its region
	partition := awsrulesfn.GetPartition(region)

	// Need to include the full repository path and the imageID (e.g. /eks/image-name:tag)
	tokens := strings.SplitN(input, "/", 2)
	if len(tokens) != 2 {
		return ECRSpec{}, errInvalidImageURI
	}

	fullRepoPath := tokens[len(tokens)-1]
	// Run simple checks on the provided repository.
	switch {
	case
		// Must not be empty
		fullRepoPath == "",
		// Must not have a partial/unsupplied label
		strings.HasSuffix(fullRepoPath, ":"),
		// Must not have a partial/unsupplied digest specifier
		strings.HasSuffix(fullRepoPath, "@"):
		return ECRSpec{}, errors.New("incomplete reference provided")
	}

	// Parse out image reference's to validate.
	ref, err := reference.Parse(repositoryPrefix + fullRepoPath)
	if err != nil {
		return ECRSpec{}, err
	}
	// If the digest is provided, check that it is valid.
	if ref.Digest() != "" {
		err := ref.Digest().Validate()
		// Digest may not be supported by the client despite it passing against
		// a rudimentary check. The error is different in the passing case, so
		// that's considered a passing check for unavailable digesters.
		//
		// https://github.com/opencontainers/go-digest/blob/ea51bea511f75cfa3ef6098cc253c5c3609b037a/digest.go#L110-L115
		if err != nil && err != digest.ErrDigestUnsupported {
			return ECRSpec{}, fmt.Errorf("%v: %w", errInvalidImageURI.Error(), err)
		}
	}

	return ECRSpec{
		Repository: strings.TrimPrefix(ref.Locator, repositoryPrefix),
		Object:     ref.Object,
		arn: arn.ARN{
			Partition: partition.Name,
			Service:   arnServiceID,
			Region:    region,
			AccountID: account,
			Resource:  ref.Locator,
		},
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

// parseARN parses an ECR ARN into its constituent parts.
//
// An example ARN is: arn:aws:ecr:us-west-2:123456789012:repository/foo/bar
func parseARN(a string) (ECRSpec, error) {
	parsed, err := arn.Parse(a)
	if err != nil {
		return ECRSpec{}, err
	}

	spec, err := reference.Parse(parsed.Resource)
	if err != nil {
		return ECRSpec{}, err
	}
	parsed.Resource = spec.Locator

	// Extract unprefixed repo name contained in the resource part.
	unprefixedRepo := strings.TrimPrefix(parsed.Resource, repositoryPrefix)
	if unprefixedRepo == parsed.Resource {
		return ECRSpec{}, invalidARN
	}

	return ECRSpec{
		arn:        parsed,
		Repository: unprefixedRepo,
		Object:     spec.Object,
	}, nil
}

// Canonical returns the canonical representation for the reference
func (spec ECRSpec) Canonical() string {
	return spec.Spec().String()
}

// ARN returns the canonical representation of the ECR ARN
func (spec ECRSpec) ARN() string {
	return spec.arn.String()
}

// Spec returns a reference.Spec
func (spec ECRSpec) Spec() reference.Spec {
	return reference.Spec{
		Locator: refPrefix + spec.ARN(),
		Object:  spec.Object,
	}
}

// ImageID returns an ecr.ImageIdentifier suitable for using in calls to ECR
func (spec ECRSpec) ImageID() *types.ImageIdentifier {
	imageID := types.ImageIdentifier{}
	tag, digest := spec.TagDigest()
	if tag != "" {
		imageID.ImageTag = aws.String(tag)
	}
	if digest != "" {
		imageID.ImageDigest = aws.String(digest.String())
	}
	return &imageID
}

// TagDigest returns the tag and/or digest specified by the reference
func (spec ECRSpec) TagDigest() (string, digest.Digest) {
	tag, digest := reference.SplitObject(spec.Object)
	return strings.TrimSuffix(tag, "@"), digest
}
