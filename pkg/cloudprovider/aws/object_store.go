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

package aws

import (
	"io"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const (
	s3URLKey            = "s3Url"
	kmsKeyIDKey         = "kmsKeyId"
	s3ForcePathStyleKey = "s3ForcePathStyle"
	bucketKey           = "bucket"
)

type s3Interface interface {
	HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	ListObjectsV2Pages(input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool) error
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	GetObjectRequest(input *s3.GetObjectInput) (req *request.Request, output *s3.GetObjectOutput)
}

type objectStore struct {
	logger     logrus.FieldLogger
	s3         s3Interface
	s3Uploader *s3manager.Uploader
	kmsKeyID   string
}

func NewObjectStore(logger logrus.FieldLogger) cloudprovider.ObjectStore {
	return &objectStore{logger: logger}
}

func (o *objectStore) Init(config map[string]string) error {
	var (
		region              = config[regionKey]
		s3URL               = config[s3URLKey]
		kmsKeyID            = config[kmsKeyIDKey]
		s3ForcePathStyleVal = config[s3ForcePathStyleKey]

		// note that bucket is automatically added to the config map
		// by the server from the ObjectStorageProviderConfig so
		// doesn't need to be explicitly set by the user within
		// config.
		bucket           = config[bucketKey]
		s3ForcePathStyle bool
		err              error
	)

	if s3ForcePathStyleVal != "" {
		if s3ForcePathStyle, err = strconv.ParseBool(s3ForcePathStyleVal); err != nil {
			return errors.Wrapf(err, "could not parse %s (expected bool)", s3ForcePathStyleKey)
		}
	}

	// AWS (not an alternate S3-compatible API) and region not
	// explicitly specified: determine the bucket's region
	if s3URL == "" && region == "" {
		var err error

		region, err = GetBucketRegion(bucket)
		if err != nil {
			return err
		}
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
		return err
	}

	o.s3 = s3.New(sess)
	o.s3Uploader = s3manager.NewUploader(sess)
	o.kmsKeyID = kmsKeyID

	return nil
}

func (o *objectStore) PutObject(bucket string, key string, body io.Reader) error {
	req := &s3manager.UploadInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   body,
	}

	// if kmsKeyID is not empty, enable "aws:kms" encryption
	if o.kmsKeyID != "" {
		req.ServerSideEncryption = aws.String("aws:kms")
		req.SSEKMSKeyId = &o.kmsKeyID
	}

	_, err := o.s3Uploader.Upload(req)

	return errors.Wrapf(err, "error putting object %s", key)
}

const notFoundCode = "NotFound"

func (o *objectStore) ObjectExists(bucket, key string) (bool, error) {
	log := o.logger.WithFields(
		logrus.Fields{
			"bucket": bucket,
			"key":    key,
		},
	)

	req := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	log.Debug("Checking if object exists")
	_, err := o.s3.HeadObject(req)

	if err == nil {
		log.Debug("Object exists")
		return true, nil
	}

	log.Debug("Checking for AWS specific error information")
	if aerr, ok := err.(awserr.Error); ok {
		log.WithFields(
			logrus.Fields{
				"code":    aerr.Code(),
				"message": aerr.Message(),
			},
		).Debugf("awserr.Error contents (origErr=%v)", aerr.OrigErr())

		// The code will be NotFound if the key doesn't exist.
		// See https://github.com/aws/aws-sdk-go/issues/1208 and https://github.com/aws/aws-sdk-go/pull/1213.
		log.Debugf("Checking for code=%s", notFoundCode)
		if aerr.Code() == notFoundCode {
			log.Debug("Object doesn't exist - got not found")
			return false, nil
		}
	}

	return false, errors.WithStack(err)
}

func (o *objectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	req := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	res, err := o.s3.GetObject(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting object %s", key)
	}

	return res.Body, nil
}

func (o *objectStore) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Delimiter: &delimiter,
	}

	var ret []string
	err := o.s3.ListObjectsV2Pages(req, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
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

func (o *objectStore) ListObjects(bucket, prefix string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	}

	var ret []string
	err := o.s3.ListObjectsV2Pages(req, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
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

func (o *objectStore) DeleteObject(bucket string, key string) error {
	req := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err := o.s3.DeleteObject(req)

	return errors.Wrapf(err, "error deleting object %s", key)
}

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	req, _ := o.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	return req.Presign(ttl)
}
