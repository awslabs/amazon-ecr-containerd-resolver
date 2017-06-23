package arn

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseARN(t *testing.T) {
	cases := []struct {
		input string
		arn   ARN
		err   error
	}{
		{
			input: "invalid",
			err:   ARNError(errors.New(invalidPrefix)),
		},
		{
			input: "arn:nope",
			err:   ARNError(errors.New(invalidSections)),
		},
		{
			input: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			arn: ARN{
				Partition: "aws",
				Service:   "ecr",
				Region:    "us-west-2",
				AccountID: "123456789012",
				Resource:  "repository/foo/bar",
			},
		},
		{
			input: "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/My App/MyEnvironment",
			arn: ARN{
				Partition: "aws",
				Service:   "elasticbeanstalk",
				Region:    "us-east-1",
				AccountID: "123456789012",
				Resource:  "environment/My App/MyEnvironment",
			},
		},
		{
			input: "arn:aws:iam::123456789012:user/David",
			arn: ARN{
				Partition: "aws",
				Service:   "iam",
				Region:    "",
				AccountID: "123456789012",
				Resource:  "user/David",
			},
		},
		{
			input: "arn:aws:rds:eu-west-1:123456789012:db:mysql-db",
			arn: ARN{
				Partition: "aws",
				Service:   "rds",
				Region:    "eu-west-1",
				AccountID: "123456789012",
				Resource:  "db:mysql-db",
			},
		},
		{
			input: "arn:aws:s3:::my_corporate_bucket/exampleobject.png",
			arn: ARN{
				Partition: "aws",
				Service:   "s3",
				Region:    "",
				AccountID: "",
				Resource:  "my_corporate_bucket/exampleobject.png",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			spec, err := Parse(tc.input)
			assert.Equal(t, tc.arn, spec)
			if tc.err == nil {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.err, err)
			}
		})
	}
}
