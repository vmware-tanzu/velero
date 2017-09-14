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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	"github.com/heptio/ark/pkg/cloudprovider"
)

var _ cloudprovider.ObjectStorageAdapter = &objectStorageAdapter{}

type objectStorageAdapter struct {
	s3       *s3.S3
	kmsKeyID string
}

func NewObjectStorageAdapter(region, s3URL, kmsKeyID string, s3ForcePathStyle bool) (cloudprovider.ObjectStorageAdapter, error) {
	if region == "" {
		return nil, errors.New("missing region in aws configuration in config file")
	}

	awsConfig := aws.NewConfig().
		WithRegion(region).
		WithS3ForcePathStyle(s3ForcePathStyle)

	if s3URL != "" {
		awsConfig = awsConfig.WithEndpointResolver(
			endpoints.ResolverFunc(func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
				if service == endpoints.S3ServiceID {
					return endpoints.ResolvedEndpoint{
						URL: s3URL,
					}, nil
				}

				return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
			}),
		)
	}

	sess, err := getSession(awsConfig)
	if err != nil {
		return nil, err
	}

	return &objectStorageAdapter{
		s3:       s3.New(sess),
		kmsKeyID: kmsKeyID,
	}, nil
}

func (op *objectStorageAdapter) PutObject(bucket string, key string, body io.ReadSeeker) error {
	req := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   body,
	}

	// if kmsKeyID is not empty, enable "aws:kms" encryption
	if op.kmsKeyID != "" {
		req.ServerSideEncryption = aws.String("aws:kms")
		req.SSEKMSKeyId = &op.kmsKeyID
	}

	_, err := op.s3.PutObject(req)

	return errors.Wrapf(err, "error putting object %s", key)
}

func (op *objectStorageAdapter) GetObject(bucket string, key string) (io.ReadCloser, error) {
	req := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	res, err := op.s3.GetObject(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting object %s", key)
	}

	return res.Body, nil
}

func (op *objectStorageAdapter) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Delimiter: &delimiter,
	}

	var ret []string
	err := op.s3.ListObjectsV2Pages(req, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, prefix := range page.CommonPrefixes {
			ret = append(ret, *prefix.Prefix)
		}
		return !lastPage
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return ret, nil
}

func (op *objectStorageAdapter) ListObjects(bucket, prefix string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	}

	var ret []string
	err := op.s3.ListObjectsV2Pages(req, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			ret = append(ret, *obj.Key)
		}
		return !lastPage
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return ret, nil
}

func (op *objectStorageAdapter) DeleteObject(bucket string, key string) error {
	req := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err := op.s3.DeleteObject(req)

	return errors.Wrapf(err, "error deleting object %s", key)
}

func (op *objectStorageAdapter) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	req, _ := op.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	return req.Presign(ttl)
}
