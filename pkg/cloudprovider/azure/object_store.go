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

package azure

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	storagemgmt "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2018-02-01/storage"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/velero/pkg/cloudprovider"
)

const (
	storageAccountConfigKey = "storageAccount"
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
	GetSASURI(options *storage.BlobSASOptions) (string, error)
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

func (b *azureBlob) GetSASURI(options *storage.BlobSASOptions) (string, error) {
	return b.blob.GetSASURI(*options)
}

type ObjectStore struct {
	containerGetter containerGetter
	blobGetter      blobGetter
	log             logrus.FieldLogger
}

func NewObjectStore(logger logrus.FieldLogger) *ObjectStore {
	return &ObjectStore{log: logger}
}

func getStorageAccountKey(config map[string]string) (string, error) {
	// load environment vars from $AZURE_CREDENTIALS_FILE, if it exists
	if err := loadEnv(); err != nil {
		return "", err
	}

	// 1. we need AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID
	envVars, err := getRequiredValues(os.Getenv, tenantIDEnvVar, clientIDEnvVar, clientSecretEnvVar, subscriptionIDEnvVar)
	if err != nil {
		return "", errors.Wrap(err, "unable to get all required environment variables")
	}

	// 2. we need config["resourceGroup"], config["storageAccount"]
	if _, err := getRequiredValues(mapLookup(config), resourceGroupConfigKey, storageAccountConfigKey); err != nil {
		return "", errors.Wrap(err, "unable to get all required config values")
	}

	// 3. get SPT
	spt, err := newServicePrincipalToken(envVars[tenantIDEnvVar], envVars[clientIDEnvVar], envVars[clientSecretEnvVar], azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return "", errors.Wrap(err, "error getting service principal token")
	}

	// 4. get storageAccountsClient
	storageAccountsClient := storagemgmt.NewAccountsClient(envVars[subscriptionIDEnvVar])
	storageAccountsClient.Authorizer = autorest.NewBearerAuthorizer(spt)

	// 5. get storage key
	res, err := storageAccountsClient.ListKeys(context.TODO(), config[resourceGroupConfigKey], config[storageAccountConfigKey])
	if err != nil {
		return "", errors.WithStack(err)
	}
	if res.Keys == nil || len(*res.Keys) == 0 {
		return "", errors.New("No storage keys found")
	}

	var storageKey string
	for _, key := range *res.Keys {
		// uppercase both strings for comparison because the ListKeys call returns e.g. "FULL" but
		// the storagemgmt.Full constant in the SDK is defined as "Full".
		if strings.ToUpper(string(key.Permissions)) == strings.ToUpper(string(storagemgmt.Full)) {
			storageKey = *key.Value
			break
		}
	}

	if storageKey == "" {
		return "", errors.New("No storage key with Full permissions found")
	}

	return storageKey, nil
}

func mapLookup(data map[string]string) func(string) string {
	return func(key string) string {
		return data[key]
	}
}

func (o *ObjectStore) Init(config map[string]string) error {
	if err := cloudprovider.ValidateObjectStoreConfigKeys(config, resourceGroupConfigKey, storageAccountConfigKey); err != nil {
		return err
	}

	storageAccountKey, err := getStorageAccountKey(config)
	if err != nil {
		return err
	}

	// 6. get storageClient and blobClient
	storageClient, err := storage.NewBasicClient(config[storageAccountConfigKey], storageAccountKey)
	if err != nil {
		return errors.Wrap(err, "error getting storage client")
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

func (o *ObjectStore) PutObject(bucket, key string, body io.Reader) error {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.CreateBlockBlobFromReader(body, nil))
}

func (o *ObjectStore) ObjectExists(bucket, key string) (bool, error) {
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

func (o *ObjectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
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

func (o *ObjectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	container, err := o.containerGetter.getContainer(bucket)
	if err != nil {
		return nil, err
	}

	params := storage.ListBlobsParameters{
		Prefix:    prefix,
		Delimiter: delimiter,
	}

	res, err := container.ListBlobs(params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return res.BlobPrefixes, nil
}

func (o *ObjectStore) ListObjects(bucket, prefix string) ([]string, error) {
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

func (o *ObjectStore) DeleteObject(bucket string, key string) error {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return err
	}

	return errors.WithStack(blob.Delete(nil))
}

func (o *ObjectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	blob, err := o.blobGetter.getBlob(bucket, key)
	if err != nil {
		return "", err
	}

	opts := storage.BlobSASOptions{
		SASOptions: storage.SASOptions{
			Expiry: time.Now().Add(ttl),
		},
		BlobServiceSASPermissions: storage.BlobServiceSASPermissions{
			Read: true,
		},
	}

	return blob.GetSASURI(&opts)
}
