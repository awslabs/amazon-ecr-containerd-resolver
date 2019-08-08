/*
 * Copyright 2017-2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package stream

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testBufferString = []string{"A", "B", "C", "D", "E", "F", "G"}
var testReaderString = "ABCDEFG"

func TestChunkedProcessorSuccess(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(testReaderString), 1, 10, func(b *Chunk) error {
		assert.Equal(t, testBufferString[index], string(b.Bytes))
		index += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, int64(6), size)
	assert.Equal(t, 7, index)
}

func TestChunkedProcessorFail(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(testReaderString), 1, 10, func(b *Chunk) error {
		index += 1
		return errors.New("error")
	})
	assert.Error(t, err)
	assert.Equal(t, int64(0), size)
	assert.Equal(t, 1, index)
}

func TestChunkedProcessorBlockingFail(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(testReaderString), 1, 2, func(b *Chunk) error {
		index += 1
		return errors.New("error")
	})
	assert.Error(t, err)
	assert.Equal(t, int64(0), size)
	assert.Equal(t, 1, index)
}

func TestChunkedProcessorBlockingSuccess(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(testReaderString), 1, 2, func(b *Chunk) error {
		assert.Equal(t, testBufferString[index], string(b.Bytes))
		index += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, int64(6), size)
	assert.Equal(t, 7, index)
}

var testChunkedString = []string{"ABC", "DEF", "G"}

func TestChunkedProcessorChunkingSuccess(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(testReaderString), 3, 2, func(b *Chunk) error {
		assert.Equal(t, testChunkedString[index], string(b.Bytes))
		index += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, int64(6), size)
	assert.Equal(t, 3, index)
}

func TestChunkedProcessorEmptySuccess(t *testing.T) {
	var index int
	size, err := ChunkedProcessor(strings.NewReader(""), 1, 2, func(b *Chunk) error {
		index += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, int64(0), size)
	assert.Equal(t, 0, index)
}
