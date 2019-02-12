/*
Copyright 2019 the Heptio Ark contributors.

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

package openstack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/objects"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/velero/pkg/cloudprovider"
)

// bucketWriter wraps the GCP SDK functions for accessing object store so they can be faked for testing.
type bucketWriter interface {
	// getWriteCloser returns an io.WriteCloser that can be used to upload data to the specified bucket for the specified key.
	getWriteCloser(bucket, key string) io.WriteCloser
}

type writer struct {
	client *storage.Client
}

func (w *writer) getWriteCloser(bucket, key string) io.WriteCloser {
	return w.client.Bucket(bucket).Object(key).NewWriter(context.Background())
}

type objectStore struct {
	client *gophercloud.ServiceClient
	log    logrus.FieldLogger
}

func NewObjectStore(logger logrus.FieldLogger) cloudprovider.ObjectStore {
	return &objectStore{log: logger}
}

func (o *objectStore) Init(config map[string]string) error {
	pc, err := authenticate()
	if err != nil {
		return errors.WithStack(err)
	}

	region := getRegion()

	client, err := openstack.NewObjectStorageV1(pc, gophercloud.EndpointOpts{
		Type:   "object-store",
		Region: region,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	o.client = client

	return nil
}

func (o *objectStore) PutObject(bucket, key string, body io.Reader) error {
	_, err := objects.Create(o.client, bucket, key, objects.CreateOpts{
		Content: body,
	}).Extract()
	return err
}

func (o *objectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
	download := objects.Download(o.client, bucket, key, objects.DownloadOpts{})
	if download.Err != nil {
		return nil, errors.WithStack(download.Err)
	}
	return download.Body, nil
}

func (o *objectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	opts := objects.ListOpts{
		Prefix:    prefix,
		Delimiter: delimiter,
		Full:      true,
	}
	var objNames []string
	pager := objects.List(o.client, bucket, opts)
	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		if objPage, ok := page.(objects.ObjectPage); ok {
			empty, _ := objPage.IsEmpty()
			if empty {
				return true, nil
			}
			var objList []objects.Object
			err := objPage.ExtractInto(&objList)
			if err != nil {
				return false, err
			}
			for _, object := range objList {
				objNames = append(objNames, object.Name)
			}
			return true, nil
		}
		return false, fmt.Errorf("Page not instance of objectPage")
	})
	if err != nil {
		return []string{}, err
	}
	return objNames, nil
}

func (o *objectStore) ListObjects(bucket, prefix string) ([]string, error) {
	return o.ListCommonPrefixes(bucket, prefix, "/")
}

func (o *objectStore) DeleteObject(bucket, key string) error {
	_, err := objects.Delete(o.client, bucket, key, objects.DeleteOpts{}).Extract()
	return err
}

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return objects.CreateTempURL(o.client, bucket, key, objects.CreateTempURLOpts{
		Method: http.MethodGet,
		TTL:    int(ttl.Seconds()),
	})
}
