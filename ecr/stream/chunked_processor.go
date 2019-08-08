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

// Package stream contains functionality for processing arbitrarily large
// streaming data.
package stream

import (
	"context"
	"io"
	"time"
)

// Chunk represents a single part of a full io stream.
type Chunk struct {
	Bytes      []byte        // buffered content
	Part       int64         // current part of io, starting at 1
	BytesBegin int64         // beginning byte range
	BytesEnd   int64         // ending byte range
	ReadTime   time.Duration // time spent reading buffer
}

type chunkedProcessor struct {
	ctx          context.Context
	cancel       func()
	readChannel  chan *Chunk
	errorChannel chan error
	reader       io.Reader
	chunkSize    int64
	queueSize    int64
}

// readCallbackFunc represents a callback function for processing chunks
type readCallbackFunc func(*Chunk) error

// ChunkedProcessor breaks an io.Reader into smaller parts (Chunks) and invokes
// callbacks on those chunks.
//
// ChunkedProcessor asynchronously reads a io.Reader into at most queueSize
// Chunks of chunkSize at a time.  The caller provides a readCallback function
// to handle each read Chunk, which does not block reading.  readCallback
// invocations are sequential and the next readCallback will not be invoked
// until the previous has completed.  If the queue of Chunks is full, the
// ChunkedProcessor will block waiting until the next readCallback is invoked
// to read from the queued Chunks.
//
// Parameters
//
// reader - the io.Reader to read.
//
// chunkSize - the maximum number of bytes that should be present in each chunk.
// All chunks except the last chunk should be exactly chunkSize.
//
// queueSize - the maximum number of unprocessed chunks to buffer.
//
// readCallback - the callback function to invoke for each chunk.
func ChunkedProcessor(reader io.Reader, chunkSize int64, queueSize int64, readCallback readCallbackFunc) (int64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	bufferedReader := &chunkedProcessor{
		ctx:          ctx,
		cancel:       cancel,
		readChannel:  make(chan *Chunk, queueSize),
		errorChannel: make(chan error),
		reader:       reader,
		chunkSize:    chunkSize,
		queueSize:    queueSize,
	}
	defer close(bufferedReader.errorChannel)

	// Drain the read channel to void leaking the readIntoChunks goroutine.
	// When we return with an error out, we may have Chunks that have been read but not yet processed.
	defer func() {
		i := 0
		for range bufferedReader.readChannel {
			i++
		}
	}()

	go bufferedReader.readIntoChunks()

	return bufferedReader.processChunks(readCallback)
}

// readIntoChunks begins event loop for reading Chunks.
// On return, either the complete buffer is read (or there is an
// error reading from the buffer) and the readChannel
//
// Can be canceled by canceling the context.
func (processor *chunkedProcessor) readIntoChunks() {
	var currentBytes, currentPart int64
	defer close(processor.readChannel)

	for {
		select {
		case <-processor.ctx.Done():
			return
		default:
			chunk, err := processor.readChunk(currentBytes, currentPart)
			if err != nil && err != io.EOF {
				processor.errorChannel <- err
				return
			}

			if chunk != nil {
				processor.readChannel <- chunk
				currentBytes = chunk.BytesEnd + 1
				currentPart++
			}

			if err != nil && err == io.EOF {
				return
			}
		}
	}

}

// processChunks selects between the read & error channels provided in the
// context and invokes the readCallback with the results on success.
//
// If an error is received in the error channel or from the read callback,
// the function returns and cancels the context.
func (processor *chunkedProcessor) processChunks(readCallback readCallbackFunc) (int64, error) {
	defer processor.cancel()

	lastReadByte := int64(0)
	eof := false

	for !eof {
		select {
		case chunk := <-processor.readChannel:
			if chunk == nil {
				eof = true
				break
			}
			lastReadByte = chunk.BytesEnd
			err := readCallback(chunk)

			if err != nil {
				return 0, err
			}
		case err := <-processor.errorChannel:
			return 0, err
		}
	}

	return lastReadByte, nil
}

// readChunk reads and returns a new Chunk to the caller.
// Given the current part and bytesBegin, populates the new Chunk with
// the proper offsets. Will return nil Chunk if reader is empty.
func (processor *chunkedProcessor) readChunk(bytesBegin int64, part int64) (*Chunk, error) {
	startTime := time.Now()
	buffer := make([]byte, processor.chunkSize)
	size, err := io.ReadFull(processor.reader, buffer)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}

	var chunk *Chunk

	if size > 0 {
		chunk = &Chunk{
			Part:       part,
			BytesBegin: bytesBegin,
			BytesEnd:   bytesBegin + int64(size) - 1,
			Bytes:      buffer[0:size],
			ReadTime:   time.Since(startTime),
		}
	}

	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}

	return chunk, err
}
