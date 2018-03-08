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
