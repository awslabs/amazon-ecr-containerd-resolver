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

package http

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

const (
	// mockURL is a fake URL modeling ecr resolver fetching content from S3.
	mockURL = "https://s3.us-east-1.amazonaws.com/981ebdad55863b3631dce86a228a3ea230dc87673a06a7d216b1275d4dd707c9/12d7153d7eee2fd595a25e5378384f1ae4b6a1658298a54c5bd3f951ec50b7cb"

	// mockQuery is a fake HTTP query with sensitive information which should be redacted.
	mockQuery = "?username=admin&password=admin"

	// redactedQuery is the expected result of redacting mockQuery.
	// The query values will be sorted by key as a side-effect of encoding the URL query string back into the URL.
	// See https://pkg.go.dev/net/url#Values.Encode
	redactedQuery = "?password=redacted&username=redacted"
)

func TestRedactHTTPQueryValuesFromURLError(t *testing.T) {
	testCases := []struct {
		Name        string
		Description string
		Err         error
		Assert      func(*testing.T, error)
	}{
		{
			Name:        "NilError",
			Description: "Utility should handle nil error gracefully",
			Err:         nil,
			Assert: func(t *testing.T, actual error) {
				if actual != nil {
					t.Fatalf("Expected nil error, got '%v'", actual)
				}
			},
		},
		{
			Name:        "NonURLError",
			Description: "Utility should not modify an error if error is not a URL error",
			Err:         errors.New("this error is not a URL error"),
			Assert: func(t *testing.T, actual error) {
				const expected = "this error is not a URL error"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
		{
			Name:        "ErrorWithNoHTTPQuery",
			Description: "Utility should not modify an error if no HTTP queries are present.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL,
				Err: errors.New("connect: connection refused"),
			},
			Assert: func(t *testing.T, actual error) {
				const expected = "GET \"" + mockURL + "\": connect: connection refused"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
		{
			Name:        "ErrorWithHTTPQuery",
			Description: "Utility should redact HTTP query values in errors to prevent logging sensitive information.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL + mockQuery,
				Err: errors.New("connect: connection refused"),
			},
			Assert: func(t *testing.T, actual error) {
				const expected = "GET \"" + mockURL + redactedQuery + "\": connect: connection refused"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			actual := RedactHTTPQueryValuesFromURLError(testCase.Err)
			testCase.Assert(t, actual)
		})
	}
}

func TestRedactHTTPQueryValuesFromURL(t *testing.T) {
	testCases := []struct {
		Name        string
		Description string
		URL         string
		Expected    string
	}{
		{
			Name:        "EmptyURL",
			Description: "Utility should gracefully handle an empty URL input",
			URL:         "",
			Expected:    "",
		},
		{
			Name:        "ValidURLWithoutQuery",
			Description: "Utility should not modify a valid URL with no HTTP query",
			URL:         mockURL,
			Expected:    mockURL,
		},
		{
			Name:        "ValidURLWithQuery",
			Description: "Utility should redact HTTP query values",
			URL:         mockURL + mockQuery,
			Expected:    mockURL + redactedQuery,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			actual := RedactHTTPQueryValuesFromURL(testCase.URL)
			if strings.Compare(testCase.Expected, actual) != 0 {
				t.Fatalf("Expected '%s', got '%s'", testCase.Expected, actual)
			}
		})
	}
}
