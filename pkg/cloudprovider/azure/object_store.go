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

package azure

import (
	"io"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/pkg/errors"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type containerGetter interface {
	getContainer(bucket string) (container, error)
}

type azureContainerGetter struct {
	blobService *storage.BlobStorageClient
}

func (cg *azureContainerGetter) getContainer(bucket string) (container, error) {
	container := cg.blobService.GetContainerReference(bucket)
	if container == nil {
		return nil, errors.Errorf("unable to get container reference for bucket %v", bucket)
	}

	return &azureContainer{
		container: container,
	}, nil
}

type container interface {
	ListBlobs(params storage.ListBlobsParameters) (storage.BlobListResponse, error)
}

type azureContainer struct {
	container *storage.Container
}

func (c *azureContainer) ListBlobs(params storage.ListBlobsParameters) (storage.BlobListResponse, error) {
	return c.container.ListBlobs(params)
}

type blobGetter interface {
	getBlob(bucket, key string) (blob, error)
}

type azureBlobGetter struct {
	blobService *storage.BlobStorageClient
}

func (bg *azureBlobGetter) getBlob(bucket, key string) (blob, error) {
	container := bg.blobService.GetContainerReference(bucket)
	if container == nil {
		return nil, errors.Errorf("unable to get container reference for bucket %v", bucket)
	}

	blob := container.GetBlobReference(key)
	if blob == nil {
		return nil, errors.Errorf("unable to get blob reference for key %v", key)
	}

	return &azureBlob{
		blob: blob,
	}, nil
}

type blob interface {
	CreateBlockBlobFromReader(blob io.Reader, options *storage.PutBlobOptions) error
	Exists() (bool, error)
	Get(options *storage.GetBlobOptions) (io.ReadCloser, error)
	Delete(options *storage.DeleteBlobOptions) error
	GetSASURI(expiry time.Time, permissions string) (string, error)
}

type azureBlob struct {
	blob *storage.Blob
}

func (b *azureBlob) CreateBlockBlobFromReader(blob io.Reader, options *storage.PutBlobOptions) error {
	return b.blob.CreateBlockBlobFromReader(blob, options)
}

func (b *azureBlob) Exists() (bool, error) {
	return b.blob.Exists()
}

func (b *azureBlob) Get(options *storage.GetBlobOptions) (io.ReadCloser, error) {
	return b.blob.Get(options)
}

func (b *azureBlob) Delete(options *storage.DeleteBlobOptions) error {
	return b.blob.Delete(options)
}

func (b *azureBlob) GetSASURI(expiry time.Time, permissions string) (string, error) {
	return b.blob.GetSASURI(expiry, permissions)
}

type objectStore struct {
	containerGetter containerGetter
	blobGetter      blobGetter
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
	o.containerGetter = &azureContainerGetter{
		blobService: &blobClient,
	}
	o.blobGetter = &azureBlobGetter{
		blobService: &blobClient,
	}

	return nil
}

func (o *objectStore) PutObject(bucket string, key string, body io.Reader) error {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.CreateBlockBlobFromReader(body, nil))
}

func (o *objectStore) ObjectExists(bucket, key string) (bool, error) {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return false, err
	}

	exists, err := blob.Exists()
	if err != nil {
		return false, errors.WithStack(err)
	}

	return exists, nil
}

func (o *objectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	blob, err := o.blobGetter.getBlob(bucket, key)
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
	container, err := o.containerGetter.getContainer(bucket)
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
	container, err := o.containerGetter.getContainer(bucket)
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
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.Delete(nil))
}

const sasURIReadPermission = "r"

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return "", err
	}

	return blob.GetSASURI(time.Now().Add(ttl), sasURIReadPermission)
}
