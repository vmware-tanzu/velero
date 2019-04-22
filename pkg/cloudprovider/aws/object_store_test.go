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

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/heptio/velero/pkg/util/test"
)

func TestIsValidSignatureVersion(t *testing.T) {
	assert.True(t, isValidSignatureVersion("1"))
	assert.True(t, isValidSignatureVersion("4"))
	assert.False(t, isValidSignatureVersion("3"))
}

type mockS3 struct {
	mock.Mock
}

func (m *mockS3) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

func (m *mockS3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *mockS3) ListObjectsV2Pages(input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool) error {
	args := m.Called(input, fn)
	return args.Error(0)
}

func (m *mockS3) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.DeleteObjectOutput), args.Error(1)
}

func (m *mockS3) GetObjectRequest(input *s3.GetObjectInput) (req *request.Request, output *s3.GetObjectOutput) {
	args := m.Called(input)
	return args.Get(0).(*request.Request), args.Get(1).(*s3.GetObjectOutput)
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
			errorResponse:  awserr.New(s3.ErrCodeNoSuchKey, "no such key", nil),
			expectedExists: false,
			expectedError:  "NoSuchKey: no such key",
		},
		{
			name:           "error checking for existence",
			errorResponse:  errors.Errorf("bad"),
			expectedExists: false,
			expectedError:  "bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := new(mockS3)
			defer s.AssertExpectations(t)

			o := &ObjectStore{
				log: test.NewLogger(),
				s3:  s,
			}

			bucket := "b"
			key := "k"
			req := &s3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}

			s.On("HeadObject", req).Return(&s3.HeadObjectOutput{}, tc.errorResponse)

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
