/*
Copyright 2017, 2019 the Velero contributors.

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
	"os"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/velero/pkg/cloudprovider"
)

const (
	regionKey = "region"
)

type bucketGetter interface {
	Bucket(bucket string) (ossBucket, error)
}

type ossBucket interface {
	IsObjectExist(key string) (bool, error)
	ListObjects(options ...oss.Option) (oss.ListObjectsResult, error)
	PutObject(objectKey string, reader io.Reader, options ...oss.Option) error
	GetObject(key string, options ...oss.Option) (io.ReadCloser, error)
	DeleteObject(key string) error
	SignURL(objectKey string, method oss.HTTPMethod, expiredInSec int64, options ...oss.Option) (string, error)
}

type ossBucketGetter struct {
	client *oss.Client
}

func (getter *ossBucketGetter) Bucket(bucket string) (ossBucket, error) {
	return getter.client.Bucket(bucket)
}

type ObjectStore struct {
	log             logrus.FieldLogger
	client          bucketGetter
	encryptionKeyID string
	privateKey      []byte
}

func NewObjectStore(logger logrus.FieldLogger) *ObjectStore {
	return &ObjectStore{log: logger}
}

func (o *ObjectStore) getBucket(bucket string) (ossBucket, error) {
	bucketObj, err := o.client.Bucket(bucket)
	if err != nil {
		o.log.Errorf("failed to get OSS bucket: %v", err)
	}
	return bucketObj, err
}

// load environment vars from $ALIBABA_CLOUD_CREDENTIALS_FILE, if it exists
func loadEnv() error {
	envFile := os.Getenv("ALIBABA_CLOUD_CREDENTIALS_FILE")
	if envFile == "" {
		return nil
	}

	if err := godotenv.Overload(envFile); err != nil {
		return errors.Wrapf(err, "error loading environment from ALIBABA_CLOUD_CREDENTIALS_FILE (%s)", envFile)
	}

	return nil
}

func (o *ObjectStore) Init(config map[string]string) error {
	if err := cloudprovider.ValidateObjectStoreConfigKeys(config); err != nil {
		return err
	}

	if err := loadEnv(); err != nil {
		return err
	}

	accessKeyID := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	stsToken := os.Getenv("ALIBABA_CLOUD_ACCESS_STS_TOKEN")
	encryptionKeyID := os.Getenv("ALIBABA_CLOUD_ENCRYPTION_KEY_ID")

	endpoint := os.Getenv("ALIBABA_CLOUD_OSS_ENDPOINT")

	if len(accessKeyID) == 0 {
		return errors.Errorf("ALIBABA_CLOUD_ACCESS_KEY_ID environment variable is not set")
	}

	if len(accessKeySecret) == 0 {
		return errors.Errorf("ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variable is not set")
	}

	if len(endpoint) == 0 {
		// Set default endpoint
		endpoint = "oss-cn-hangzhou.aliyuncs.com"
	}

	var client *oss.Client
	var err error

	if len(stsToken) == 0 {
		client, err = oss.New(endpoint, accessKeyID, accessKeySecret)
	} else {
		client, err = oss.New(endpoint, accessKeyID, accessKeySecret, oss.SecurityToken(stsToken))
	}

	if err != nil {
		return errors.Errorf("failed to create OSS client: %v", err.Error())
	}

	o.client = &ossBucketGetter{
		client,
	}

	o.encryptionKeyID = encryptionKeyID

	return nil
}

func (o *ObjectStore) PutObject(bucket, key string, body io.Reader) error {
	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return err
	}

	if o.encryptionKeyID != "" {
		err = bucketObj.PutObject(key, body,
			oss.ServerSideEncryption("KMS"),
			oss.ServerSideEncryptionKeyID(o.encryptionKeyID))
	} else {
		err = bucketObj.PutObject(key, body)
	}

	return err
}

func (o *ObjectStore) ObjectExists(bucket, key string) (bool, error) {
	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return false, err
	}
	return bucketObj.IsObjectExist(key)
}

func (o *ObjectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return nil, err
	}

	return bucketObj.GetObject(key)

}

func (o *ObjectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {

	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return nil, err
	}
	var res []string
	marker := oss.Marker("")
	for {
		lor, err := bucketObj.ListObjects(oss.Prefix(prefix), oss.Delimiter(delimiter), oss.MaxKeys(50), marker)
		if err != nil {
			o.log.Errorf("failed to list objects: %v", err)
			return res, err
		}
		res = append(res, lor.CommonPrefixes...)
		if lor.IsTruncated {
			marker = oss.Marker(lor.NextMarker)
		} else {
			break
		}
	}

	return res, nil
}

func (o *ObjectStore) ListObjects(bucket, prefix string) ([]string, error) {

	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return nil, err
	}

	var res []string
	marker := oss.Marker("")
	for {
		lor, err := bucketObj.ListObjects(oss.Prefix(prefix), oss.MaxKeys(50), marker)
		if err != nil {
			o.log.Errorf("failed to list objects: %v", err)
		}
		for _, obj := range lor.Objects {
			res = append(res, obj.Key)
		}
		if lor.IsTruncated {
			marker = oss.Marker(lor.NextMarker)
		} else {
			break
		}
	}

	return res, nil
}

func (o *ObjectStore) DeleteObject(bucket, key string) error {
	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return err
	}
	return bucketObj.DeleteObject(key)
}

func (o *ObjectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	bucketObj, err := o.getBucket(bucket)
	if err != nil {
		return "", err
	}

	return bucketObj.SignURL(key, oss.HTTPGet, int64(ttl.Seconds()))

}
