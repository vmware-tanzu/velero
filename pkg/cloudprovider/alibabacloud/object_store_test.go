/*
Copyright 2018, 2019 the Velero contributors.

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

package alibabacloud

import (
	"io"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockBucketGetter struct {
	mock.Mock
}

func (m *mockBucketGetter) Bucket(bucket string) (ossBucket, error) {
	args := m.Called(bucket)
	return args.Get(0).(ossBucket), args.Error(1)
}

type mockBucket struct {
	mock.Mock
}

func (m *mockBucket) IsObjectExist(key string) (bool, error) {
	args := m.Called(key)
	return args.Get(0).(bool), args.Error(1)
}

func (m *mockBucket) ListObjects(options ...oss.Option) (oss.ListObjectsResult, error) {
	args := m.Called(options)
	return args.Get(0).(oss.ListObjectsResult), args.Error(1)
}

func (m *mockBucket) GetObject(key string, options ...oss.Option) (io.ReadCloser, error) {
	args := m.Called(key, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockBucket) PutObject(key string, reader io.Reader, options ...oss.Option) error {
	args := m.Called(key, reader, options)
	return args.Error(0)
}

func (m *mockBucket) DeleteObject(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *mockBucket) SignURL(objectKey string, method oss.HTTPMethod, expiredInSec int64, options ...oss.Option) (string, error) {
	args := m.Called(objectKey, method, expiredInSec, options)
	return args.Get(0).(string), args.Error(1)
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
			errorResponse:  errors.New("not found"),
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
			client := new(mockBucketGetter)
			defer client.AssertExpectations(t)

			o := &ObjectStore{
				client: client,
			}

			bucket := new(mockBucket)
			defer bucket.AssertExpectations(t)

			client.On("Bucket", "bucket").Return(bucket, nil)
			bucket.On("IsObjectExist", "key").Return(tc.expectedExists, tc.errorResponse)

			exists, err := o.ObjectExists("bucket", "key")

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			assert.Equal(t, tc.expectedExists, exists)
		})
	}
}
