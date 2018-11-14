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
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const (
	s3URLKey            = "s3Url"
	publicURLKey        = "publicUrl"
	kmsKeyIDKey         = "kmsKeyId"
	s3ForcePathStyleKey = "s3ForcePathStyle"
	bucketKey           = "bucket"
)

type objectStore struct {
	log        logrus.FieldLogger
	s3         *s3.S3
	preSignS3  *s3.S3
	s3Uploader *s3manager.Uploader
	kmsKeyID   string
}

func NewObjectStore(logger logrus.FieldLogger) cloudprovider.ObjectStore {
	return &objectStore{log: logger}
}

func (o *objectStore) Init(config map[string]string) error {
	var (
		region              = config[regionKey]
		s3URL               = config[s3URLKey]
		publicURL           = config[publicURLKey]
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

	serverConfig, err := newAWSConfig(s3URL, region, s3ForcePathStyle)
	if err != nil {
		return err
	}

	serverSession, err := getSession(serverConfig)
	if err != nil {
		return err
	}

	o.s3 = s3.New(serverSession)
	o.s3Uploader = s3manager.NewUploader(serverSession)
	o.kmsKeyID = kmsKeyID

	if publicURL != "" {
		publicConfig, err := newAWSConfig(publicURL, region, s3ForcePathStyle)
		if err != nil {
			return err
		}
		publicSession, err := getSession(publicConfig)
		if err != nil {
			return err
		}
		o.preSignS3 = s3.New(publicSession)
	} else {
		o.preSignS3 = o.s3
	}

	return nil
}

func newAWSConfig(url, region string, forcePathStyle bool) (*aws.Config, error) {
	awsConfig := aws.NewConfig().
		WithRegion(region).
		WithS3ForcePathStyle(forcePathStyle)

	if url != "" {
		if !IsValidS3URLScheme(url) {
			return nil, errors.Errorf("Invalid s3 url: %s", url)
		}

		awsConfig = awsConfig.WithEndpointResolver(
			endpoints.ResolverFunc(func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
				if service == endpoints.S3ServiceID {
					return endpoints.ResolvedEndpoint{
						URL: url,
					}, nil
				}

				return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
			}),
		)
	}

	return awsConfig, nil
}

func (o *objectStore) PutObject(bucket, key string, body io.Reader) error {
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

func (o *objectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
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

func (o *objectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	req := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    &prefix,
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

	// ensure that returned objects are in a consistent order so that the deletion logic deletes the objects before
	// the pseudo-folder prefix object for s3 providers (such as Quobyte) that return the pseudo-folder as an object.
	// See https://github.com/heptio/ark/pull/999
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))

	return ret, nil
}

func (o *objectStore) DeleteObject(bucket, key string) error {
	req := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err := o.s3.DeleteObject(req)

	return errors.Wrapf(err, "error deleting object %s", key)
}

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	req, _ := o.preSignS3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	return req.Presign(ttl)
}
