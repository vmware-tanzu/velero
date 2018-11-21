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
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

type mockS3Uploader struct {
	mock.Mock
}

func (m *mockS3Uploader) Upload(req *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	args := m.Called(req, options)
	return1 := args.Get(0)
	if return1 != nil {
		return return1.(*s3manager.UploadOutput), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestPutObject(t *testing.T) {
	s3Client := &mockS3{}
	defer s3Client.AssertExpectations(t)

	s3Uploader := &mockS3Uploader{}
	defer s3Uploader.AssertExpectations(t)

	o := &objectStore{
		s3:         s3Client,
		s3Uploader: s3Uploader,
	}

	bucket := "mybucket"
	key := "mykey"
	body := strings.NewReader("mybody")

	req := &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	var noOptions []func(*s3manager.Uploader) = nil
	s3Uploader.On("Upload", req, noOptions).Return(nil, nil).Once()

	err := o.PutObject(bucket, key, body)
	assert.NoError(t, err)

	// make sure kmsKeyID is honored
	o.kmsKeyID = "mykey"
	reqWithKMS := &s3manager.UploadInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		Body:                 body,
		ServerSideEncryption: aws.String("aws:kms"),
		SSEKMSKeyId:          aws.String(o.kmsKeyID),
	}
	s3Uploader.On("Upload", reqWithKMS, noOptions).Return(nil, nil).Once()

	err = o.PutObject(bucket, key, body)
	assert.NoError(t, err)

	// make sure the error is returned
	s3Uploader.On("Upload", reqWithKMS, noOptions).Return(nil, errors.New("foo"))
	err = o.PutObject(bucket, key, body)
	assert.EqualError(t, err, "error putting object mykey: foo")
}
