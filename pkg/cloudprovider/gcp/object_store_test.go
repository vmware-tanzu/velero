/*
Copyright 2018 the Velero contributors.

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
package gcp

import (
	"errors"
	"io"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	velerotest "github.com/heptio/velero/pkg/test"
)

type mockWriteCloser struct {
	closeErr error
	writeErr error
}

func (m *mockWriteCloser) Close() error {
	return m.closeErr
}

func (m *mockWriteCloser) Write(b []byte) (int, error) {
	return len(b), m.writeErr
}

func newMockWriteCloser(writeErr, closeErr error) *mockWriteCloser {
	return &mockWriteCloser{writeErr: writeErr, closeErr: closeErr}
}

type fakeWriter struct {
	wc *mockWriteCloser

	attrsErr error
}

func newFakeWriter(wc *mockWriteCloser) *fakeWriter {
	return &fakeWriter{wc: wc}
}

func (fw *fakeWriter) getWriteCloser(bucket, name string) io.WriteCloser {
	return fw.wc
}

func (fw *fakeWriter) getAttrs(bucket, key string) (*storage.ObjectAttrs, error) {
	return new(storage.ObjectAttrs), fw.attrsErr
}

func TestPutObject(t *testing.T) {
	tests := []struct {
		name        string
		writeErr    error
		closeErr    error
		expectedErr error
	}{
		{
			name:        "No errors returns nil",
			closeErr:    nil,
			writeErr:    nil,
			expectedErr: nil,
		},
		{
			name:        "Close() errors are returned",
			closeErr:    errors.New("error closing"),
			expectedErr: errors.New("error closing"),
		},
		{
			name:        "Write() errors are returned",
			writeErr:    errors.New("error writing"),
			expectedErr: errors.New("error writing"),
		},
		{
			name:        "Write errors supercede close errors",
			writeErr:    errors.New("error writing"),
			closeErr:    errors.New("error closing"),
			expectedErr: errors.New("error writing"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wc := newMockWriteCloser(test.writeErr, test.closeErr)
			o := NewObjectStore(velerotest.NewLogger())
			o.bucketWriter = newFakeWriter(wc)

			err := o.PutObject("bucket", "key", strings.NewReader("contents"))
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestObjectExists(t *testing.T) {
	tests := []struct {
		name           string
		errorResponse  error
		expectedExists bool
		expectedError  string
	}{
		{
			name:           "exists",
			errorResponse:  nil,
			expectedExists: true,
		},
		{
			name:           "doesn't exist",
			errorResponse:  storage.ErrObjectNotExist,
			expectedExists: false,
		},
		{
			name:           "error checking for existence",
			errorResponse:  errors.New("bad"),
			expectedExists: false,
			expectedError:  "bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := NewObjectStore(velerotest.NewLogger())
			w := newFakeWriter(nil)
			o.bucketWriter = w
			w.attrsErr = tc.errorResponse

			bucket := "b"
			key := "k"
			exists, err := o.ObjectExists(bucket, key)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.expectedExists, exists)
		})
	}
}
