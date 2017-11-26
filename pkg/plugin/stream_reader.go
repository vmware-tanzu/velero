/*
Copyright 2017 the Heptio Ark contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"bytes"
	"io"
)

// ReceiveFunc is a function that either returns a slice
// of an arbitrary number of bytes OR an error. Returning
// an io.EOF means there is no more data to be read; any
// other error is considered an actual error.
type ReceiveFunc func() ([]byte, error)

// CloseFunc is used to signal to the source of data that
// the StreamReadCloser has been closed.
type CloseFunc func() error

// StreamReadCloser wraps a ReceiveFunc and a CloseSendFunc
// to implement io.ReadCloser.
type StreamReadCloser struct {
	buf     *bytes.Buffer
	receive ReceiveFunc
	close   CloseFunc
}

func (s *StreamReadCloser) Read(p []byte) (n int, err error) {
	for {
		// if buf exists and holds at least as much as we're trying to read,
		// read from the buffer
		if s.buf != nil && s.buf.Len() >= len(p) {
			return s.buf.Read(p)
		}

		// if buf is nil, create it
		if s.buf == nil {
			s.buf = new(bytes.Buffer)
		}

		// buf exists but doesn't hold enough data to fill p, so
		// receive again. If we get an EOF, return what's in the
		// buffer; else, write the new data to the buffer and
		// try another read.
		data, err := s.receive()
		if err == io.EOF {
			return s.buf.Read(p)
		}
		if err != nil {
			return 0, err
		}

		if _, err := s.buf.Write(data); err != nil {
			return 0, err
		}
	}
}

func (s *StreamReadCloser) Close() error {
	return s.close()
}
