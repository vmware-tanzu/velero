/*
Copyright 2017 Heptio Inc.

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
	"io"

	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/heptio/ark/pkg/cloudprovider"
)

var _ cloudprovider.ObjectStorageAdapter = &objectStorageAdapter{}

type objectStorageAdapter struct {
	s3 *s3.S3
}

func (op *objectStorageAdapter) PutObject(bucket string, key string, body io.ReadSeeker) error {
	req := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   body,
	}

	_, err := op.s3.PutObject(req)

	return err
}

func (op *objectStorageAdapter) GetObject(bucket string, key string) (io.ReadCloser, error) {
	req := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	res, err := op.s3.GetObject(req)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}

func (op *objectStorageAdapter) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Delimiter: &delimiter,
	}

	res, err := op.s3.ListObjectsV2(req)
	if err != nil {
		return nil, err
	}

	ret := make([]string, 0, len(res.CommonPrefixes))

	for _, prefix := range res.CommonPrefixes {
		ret = append(ret, *prefix.Prefix)
	}

	return ret, nil
}

func (op *objectStorageAdapter) DeleteObject(bucket string, key string) error {
	req := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err := op.s3.DeleteObject(req)

	return err
}
