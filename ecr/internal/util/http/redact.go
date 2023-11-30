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
)

// RedactHTTPQueryValuesFromURLError is a log utility to parse an error as a URL
// error and redact HTTP query values to prevent leaking sensitive information
// like encoded credentials or tokens.
func RedactHTTPQueryValuesFromURLError(err error) error {
	var urlErr *url.Error

	if err != nil && errors.As(err, &urlErr) {
		urlErr.URL = RedactHTTPQueryValuesFromURL(urlErr.URL)
		return urlErr
	}

	return err
}

// RedactHTTPQueryValuesFromURL is a log utility to parse a raw URL as a URL
// and redact HTTP query values to prevent leaking sensitive information
// like encoded credentials or tokens.
func RedactHTTPQueryValuesFromURL(rawURL string) string {
	url, urlParseErr := url.Parse(rawURL)
	if urlParseErr == nil && url != nil {
		if query := url.Query(); len(query) > 0 {
			for k := range query {
				query.Set(k, "redacted")
			}
			url.RawQuery = query.Encode()
		}
		return url.Redacted()
	}
	return rawURL
}
