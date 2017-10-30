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

package openstack

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/objects"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const (
	osInsecureArg = "OS_INSECURE"
)

type objectStorageAdapter struct {
	swiftClient *gophercloud.ServiceClient
}

var _ cloudprovider.ObjectStorageAdapter = &objectStorageAdapter{}

func NewObjectStorageAdapter(region string) (cloudprovider.ObjectStorageAdapter, error) {
	provider, err := getProvider()
	if err != nil {
		return nil, err
	}

	swiftClient, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not set up Swift client")
	}

	return &objectStorageAdapter{swiftClient}, nil
}

func getProvider() (*gophercloud.ProviderClient, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve OpenStack credentials from env")
	}

	provider, err := openstack.NewClient(opts.IdentityEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not set up OpenStack client")
	}

	if os.Getenv(osInsecureArg) != "" {
		provider.HTTPClient = http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	if err := openstack.Authenticate(provider, opts); err != nil {
		return nil, errors.Wrap(err, "could not authenticate to OpenStack")
	}

	return provider, nil
}

func (op *objectStorageAdapter) PutObject(bucket string, key string, body io.ReadSeeker) error {
	createOpts := objects.CreateOpts{
		Content: body,
	}
	_, err := objects.Create(op.swiftClient, bucket, key, createOpts).Extract()
	return err
}

func (op *objectStorageAdapter) GetObject(bucket string, key string) (io.ReadCloser, error) {
	res := objects.Download(op.swiftClient, bucket, key, nil)
	return res.Body, res.Err
}

func (op *objectStorageAdapter) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	// although Swift supports a `delimiter` query arg, its behaviour is quite unexpected. when provided it
	// will return anything containing that delimiter then strip the prefix. so instead we need to
	// return everything, then extract the prefix.

	allPages, err := objects.List(op.swiftClient, bucket, nil).AllPages()
	if err != nil {
		return nil, err
	}

	objectNames, err := objects.ExtractNames(allPages)
	if err != nil {
		return nil, err
	}

	objectPrefixes := []string{}

	for _, objectName := range objectNames {
		prefix := objectName[:strings.LastIndex(objectName, delimiter)]
		objectPrefixes = append(objectPrefixes, prefix)
	}

	return objectPrefixes, nil
}

func (op *objectStorageAdapter) ListObjects(bucket, prefix string) ([]string, error) {
	allPages, err := objects.List(op.swiftClient, bucket, objects.ListOpts{Prefix: prefix}).AllPages()
	if err != nil {
		return nil, err
	}

	return objects.ExtractNames(allPages)
}

func (op *objectStorageAdapter) DeleteObject(bucket string, key string) error {
	_, err := objects.Delete(op.swiftClient, bucket, key, nil).Extract()
	return err
}

func (op *objectStorageAdapter) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	urlOpts := objects.CreateTempURLOpts{
		Method: objects.GET,
		TTL:    int(ttl.Seconds()),
	}
	return objects.CreateTempURL(op.swiftClient, bucket, key, urlOpts)
}
