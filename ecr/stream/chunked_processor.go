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

type chunkedReader struct {
	ctx          context.Context
	cancel       func()
	readChannel  chan *Chunk
	errorChannel chan error
	reader       io.Reader
	chunkSize    int64
	queueSize    int64
}

type readCallbackFunc func(*Chunk) error

// ChunkedProcessor asynchronously reads a io.Reader into at most queueSize
// Chunks of chunkSize at a time.  The caller provides a readCallback function
// to handle each read Chunk, which does not block reading.  readCallback
// invocations are sequential and the next readCallback will not be invoked
// until the previous has completed.  If the queue of Chunks is full, the
// ChunkedProcessor will block waiting until the next readCallback is invoked
// to read from the queued Chunks.
func ChunkedProcessor(reader io.Reader, chunkSize int64, queueSize int64, readCallback readCallbackFunc) (int64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	bufferedReader := &chunkedReader{
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
func (reader *chunkedReader) readIntoChunks() {
	var currentBytes, currentPart int64
	defer close(reader.readChannel)

	for {
		select {
		case <-reader.ctx.Done():
			return
		default:
			chunk, err := reader.readChunk(currentBytes, currentPart)
			if err != nil && err != io.EOF {
				reader.errorChannel <- err
				return
			}

			if chunk != nil {
				reader.readChannel <- chunk
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
func (reader *chunkedReader) processChunks(readCallback readCallbackFunc) (int64, error) {
	defer reader.cancel()

	lastReadByte := int64(0)
	eof := false

	for !eof {
		select {
		case chunk := <-reader.readChannel:
			if chunk == nil {
				eof = true
				break
			}
			lastReadByte = chunk.BytesEnd
			err := readCallback(chunk)

			if err != nil {
				return 0, err
			}
		case err := <-reader.errorChannel:
			return 0, err
		}
	}

	return lastReadByte, nil
}

// readChunk reads and returns a new Chunk to the caller.
// Given the current part and bytesBegin, populates the new Chunk with
// the proper offsets. Will return nil Chunk if reader is empty.
func (reader *chunkedReader) readChunk(bytesBegin int64, part int64) (*Chunk, error) {
	startTime := time.Now()
	buffer := make([]byte, reader.chunkSize)
	size, err := io.ReadFull(reader.reader, buffer)
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
