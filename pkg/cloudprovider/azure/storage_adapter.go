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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/disk"
	"github.com/Azure/azure-sdk-for-go/arm/examples/helpers"
	"github.com/Azure/azure-sdk-for-go/arm/resources/subscriptions"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const (
	azureClientIDKey         string = "AZURE_CLIENT_ID"
	azureClientSecretKey     string = "AZURE_CLIENT_SECRET"
	azureSubscriptionIDKey   string = "AZURE_SUBSCRIPTION_ID"
	azureTenantIDKey         string = "AZURE_TENANT_ID"
	azureStorageAccountIDKey string = "AZURE_STORAGE_ACCOUNT_ID"
	azureStorageKeyKey       string = "AZURE_STORAGE_KEY"
	azureResourceGroupKey    string = "AZURE_RESOURCE_GROUP"
)

type storageAdapter struct {
	objectStorage *objectStorageAdapter
	blockStorage  *blockStorageAdapter
}

var _ cloudprovider.StorageAdapter = &storageAdapter{}

func NewStorageAdapter(location string, apiTimeout time.Duration) (cloudprovider.StorageAdapter, error) {
	cfg := map[string]string{
		azureClientIDKey:         "",
		azureClientSecretKey:     "",
		azureSubscriptionIDKey:   "",
		azureTenantIDKey:         "",
		azureStorageAccountIDKey: "",
		azureStorageKeyKey:       "",
		azureResourceGroupKey:    "",
	}

	for key := range cfg {
		cfg[key] = os.Getenv(key)
	}

	spt, err := helpers.NewServicePrincipalTokenFromCredentials(cfg, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error creating new service principal: %v", err)
	}

	disksClient := disk.NewDisksClient(cfg[azureSubscriptionIDKey])
	snapsClient := disk.NewSnapshotsClient(cfg[azureSubscriptionIDKey])

	disksClient.Authorizer = spt
	snapsClient.Authorizer = spt

	storageClient, _ := storage.NewBasicClient(cfg[azureStorageAccountIDKey], cfg[azureStorageKeyKey])
	blobClient := storageClient.GetBlobService()

	if apiTimeout == 0 {
		apiTimeout = time.Minute
	}

	// validate the location
	groupClient := subscriptions.NewGroupClient()
	groupClient.Authorizer = spt

	locs, err := groupClient.ListLocations(cfg[azureSubscriptionIDKey])
	if err != nil {
		return nil, err
	}

	if locs.Value == nil {
		return nil, errors.New("no locations returned from Azure API")
	}

	locationExists := false
	for _, loc := range *locs.Value {
		if (loc.Name != nil && *loc.Name == location) || (loc.DisplayName != nil && *loc.DisplayName == location) {
			locationExists = true
			break
		}
	}

	if !locationExists {
		return nil, fmt.Errorf("location %q not found", location)
	}

	return &storageAdapter{
		objectStorage: &objectStorageAdapter{
			blobClient: &blobClient,
		},
		blockStorage: &blockStorageAdapter{
			disks:         &disksClient,
			snaps:         &snapsClient,
			subscription:  cfg[azureSubscriptionIDKey],
			resourceGroup: cfg[azureResourceGroupKey],
			location:      location,
			apiTimeout:    apiTimeout,
		},
	}, nil
}

func (op *storageAdapter) ObjectStorage() cloudprovider.ObjectStorageAdapter {
	return op.objectStorage
}

func (op *storageAdapter) BlockStorage() cloudprovider.BlockStorageAdapter {
	return op.blockStorage
}
