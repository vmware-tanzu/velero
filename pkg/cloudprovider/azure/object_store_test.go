/*
Copyright 2018 the Heptio Ark contributors.

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

package azure

import (
	"io"
	"testing"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestObjectExists(t *testing.T) {
	tests := []struct {
		name           string
		getBlobError   error
		exists         bool
		errorResponse  error
		expectedExists bool
		expectedError  string
	}{
		{
			name:           "getBlob error",
			exists:         false,
			errorResponse:  errors.New("getBlob"),
			expectedExists: false,
			expectedError:  "getBlob",
		},
		{
			name:           "exists",
			exists:         true,
			errorResponse:  nil,
			expectedExists: true,
		},
		{
			name:           "doesn't exist",
			exists:         false,
			errorResponse:  nil,
			expectedExists: false,
		},
		{
			name:           "error checking for existence",
			exists:         false,
			errorResponse:  errors.New("bad"),
			expectedExists: false,
			expectedError:  "bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blobGetter := new(mockBlobGetter)
			defer blobGetter.AssertExpectations(t)

			o := &ObjectStore{
				blobGetter: blobGetter,
			}

			bucket := "b"
			key := "k"

			blob := new(mockBlob)
			defer blob.AssertExpectations(t)
			blobGetter.On("getBlob", bucket, key).Return(blob, tc.getBlobError)

			blob.On("Exists").Return(tc.exists, tc.errorResponse)

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

type mockBlobGetter struct {
	mock.Mock
}

func (m *mockBlobGetter) getBlob(bucket string, key string) (blob, error) {
	args := m.Called(bucket, key)
	return args.Get(0).(blob), args.Error(1)
}

type mockBlob struct {
	mock.Mock
}

func (m *mockBlob) CreateBlockBlobFromReader(blob io.Reader, options *storage.PutBlobOptions) error {
	args := m.Called(blob, options)
	return args.Error(0)
}

func (m *mockBlob) Exists() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *mockBlob) Get(options *storage.GetBlobOptions) (io.ReadCloser, error) {
	args := m.Called(options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *mockBlob) Delete(options *storage.DeleteBlobOptions) error {
	args := m.Called(options)
	return args.Error(0)
}

func (m *mockBlob) GetSASURI(options *storage.BlobSASOptions) (string, error) {
	args := m.Called(options)
	return args.String(0), args.Error(1)
}

type mockContainerGetter struct {
	mock.Mock
}

func (m *mockContainerGetter) getContainer(bucket string) (container, error) {
	args := m.Called(bucket)
	return args.Get(0).(container), args.Error(1)
}

type mockContainer struct {
	mock.Mock
}

func (m *mockContainer) ListBlobs(params storage.ListBlobsParameters) (storage.BlobListResponse, error) {
	args := m.Called(params)
	return args.Get(0).(storage.BlobListResponse), args.Error(1)
}
