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
	"os"
	"strings"
	"time"

	storagemgmt "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-10-01/storage"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const (
	storageAccountConfigKey = "storageAccount"
)

type objectStore struct {
	blobClient *storage.BlobStorageClient
	log        logrus.FieldLogger
}

func NewObjectStore(logger logrus.FieldLogger) cloudprovider.ObjectStore {
	return &objectStore{log: logger}
}

func getStorageAccountKey(config map[string]string) (string, error) {
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
	res, err := storageAccountsClient.ListKeys(config[resourceGroupConfigKey], config[storageAccountConfigKey])
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

func (o *objectStore) Init(config map[string]string) error {
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
	o.blobClient = &blobClient

	return nil
}

func (o *objectStore) PutObject(bucket, key string, body io.Reader) error {
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

func (o *objectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
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

func (o *objectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	container, err := getContainerReference(o.blobClient, bucket)
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

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	container, err := getContainerReference(o.blobClient, bucket)
	if err != nil {
		return "", err
	}

	blob, err := getBlobReference(container, key)
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

	return blob.GetSASURI(opts)
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
