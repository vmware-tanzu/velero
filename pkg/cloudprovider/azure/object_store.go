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

package azure

import (
	"io"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/pkg/errors"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type objectStore struct {
	blobClient *storage.BlobStorageClient
}

func NewObjectStore() cloudprovider.ObjectStore {
	return &objectStore{}
}

func (o *objectStore) Init(config map[string]string) error {
	cfg := getConfig()

	storageClient, err := storage.NewBasicClient(cfg[azureStorageAccountIDKey], cfg[azureStorageKeyKey])
	if err != nil {
		return errors.WithStack(err)
	}

	blobClient := storageClient.GetBlobService()

	o.blobClient = &blobClient

	return nil
}

func (o *objectStore) PutObject(bucket string, key string, body io.Reader) error {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return err
	}

	blob, err := getBlobReference(container, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.CreateBlockBlobFromReader(body, nil))
}

func (o *objectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return nil, err
	}

	blob, err := getBlobReference(container, key)
	if err != nil {
		return nil, err
	}

	res, err := blob.Get(nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return res, nil
}

func (o *objectStore) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return nil, err
	}

	params := storage.ListBlobsParameters{
		Delimiter: delimiter,
	}

	res, err := container.ListBlobs(params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Azure returns prefixes inclusive of the last delimiter. We need to strip
	// it.
	ret := make([]string, 0, len(res.BlobPrefixes))
	for _, prefix := range res.BlobPrefixes {
		ret = append(ret, prefix[0:strings.LastIndex(prefix, delimiter)])
	}

	return ret, nil
}

func (o *objectStore) ListObjects(bucket, prefix string) ([]string, error) {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return nil, err
	}

	params := storage.ListBlobsParameters{
		Prefix: prefix,
	}

	res, err := container.ListBlobs(params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ret := make([]string, 0, len(res.Blobs))
	for _, blob := range res.Blobs {
		ret = append(ret, blob.Name)
	}

	return ret, nil
}

func (o *objectStore) DeleteObject(bucket string, key string) error {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return err
	}

	blob, err := getBlobReference(container, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.Delete(nil))
}

const sasURIReadPermission = "r"

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return "", err
	}

	blob, err := getBlobReference(container, key)
	if err != nil {
		return "", err
	}

	return blob.GetSASURI(time.Now().Add(ttl), sasURIReadPermission)
}

func getContainerReference(blobClient *storage.BlobStorageClient, bucket string) (*storage.Container, error) {
	container := blobClient.GetContainerReference(bucket)
	if container == nil {
		return nil, errors.Errorf("unable to get container reference for bucket %v", bucket)
	}

	return container, nil
}

func getBlobReference(container *storage.Container, key string) (*storage.Blob, error) {
	blob := container.GetBlobReference(key)
	if blob == nil {
		return nil, errors.Errorf("unable to get blob reference for key %v", key)
	}

	return blob, nil
}
