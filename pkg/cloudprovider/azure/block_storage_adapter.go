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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/disk"
	"github.com/Azure/azure-sdk-for-go/arm/examples/helpers"
	"github.com/Azure/azure-sdk-for-go/arm/resources/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"github.com/satori/uuid"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type blockStorageAdapter struct {
	disks         *disk.DisksClient
	snaps         *disk.SnapshotsClient
	subscription  string
	resourceGroup string
	location      string
	apiTimeout    time.Duration
}

var _ cloudprovider.BlockStorageAdapter = &blockStorageAdapter{}

const (
	azureClientIDKey         string = "AZURE_CLIENT_ID"
	azureClientSecretKey     string = "AZURE_CLIENT_SECRET"
	azureSubscriptionIDKey   string = "AZURE_SUBSCRIPTION_ID"
	azureTenantIDKey         string = "AZURE_TENANT_ID"
	azureStorageAccountIDKey string = "AZURE_STORAGE_ACCOUNT_ID"
	azureStorageKeyKey       string = "AZURE_STORAGE_KEY"
	azureResourceGroupKey    string = "AZURE_RESOURCE_GROUP"
)

func getConfig() map[string]string {
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

	return cfg
}

func NewBlockStorageAdapter(location string, apiTimeout time.Duration) (cloudprovider.BlockStorageAdapter, error) {
	if location == "" {
		return nil, errors.New("missing location in azure configuration in config file")
	}

	if apiTimeout == 0 {
		apiTimeout = 2 * time.Minute
	}

	cfg := getConfig()

	spt, err := helpers.NewServicePrincipalTokenFromCredentials(cfg, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "error creating new service principal token")
	}

	disksClient := disk.NewDisksClient(cfg[azureSubscriptionIDKey])
	snapsClient := disk.NewSnapshotsClient(cfg[azureSubscriptionIDKey])

	authorizer := autorest.NewBearerAuthorizer(spt)
	disksClient.Authorizer = authorizer
	snapsClient.Authorizer = authorizer

	// validate the location
	groupClient := subscriptions.NewGroupClient()
	groupClient.Authorizer = authorizer

	locs, err := groupClient.ListLocations(cfg[azureSubscriptionIDKey])
	if err != nil {
		return nil, errors.WithStack(err)
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
		return nil, errors.Errorf("location %q not found", location)
	}

	return &blockStorageAdapter{
		disks:         &disksClient,
		snaps:         &snapsClient,
		subscription:  cfg[azureSubscriptionIDKey],
		resourceGroup: cfg[azureResourceGroupKey],
		location:      location,
		apiTimeout:    apiTimeout,
	}, nil
}

func (op *blockStorageAdapter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	fullSnapshotName := getFullSnapshotName(op.subscription, op.resourceGroup, snapshotID)
	diskName := "restore-" + uuid.NewV4().String()

	disk := disk.Model{
		Name:     &diskName,
		Location: &op.location,
		Properties: &disk.Properties{
			CreationData: &disk.CreationData{
				CreateOption:     disk.Copy,
				SourceResourceID: &fullSnapshotName,
			},
			AccountType: disk.StorageAccountTypes(volumeType),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), op.apiTimeout)
	defer cancel()

	_, errChan := op.disks.CreateOrUpdate(op.resourceGroup, *disk.Name, disk, ctx.Done())

	err := <-errChan

	if err != nil {
		return "", errors.WithStack(err)
	}
	return diskName, nil
}

func (op *blockStorageAdapter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	res, err := op.disks.Get(op.resourceGroup, volumeID)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	return string(res.AccountType), nil, nil
}

func (op *blockStorageAdapter) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	res, err := op.disks.Get(op.resourceGroup, volumeID)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if res.ProvisioningState == nil {
		return false, errors.New("nil ProvisioningState returned from Get call")
	}

	return *res.ProvisioningState == "Succeeded", nil
}

func (op *blockStorageAdapter) ListSnapshots(tagFilters map[string]string) ([]string, error) {
	res, err := op.snaps.ListByResourceGroup(op.resourceGroup)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if res.Value == nil {
		return nil, errors.New("nil Value returned from ListByResourceGroup call")
	}

	ret := make([]string, 0, len(*res.Value))
Snapshot:
	for _, snap := range *res.Value {
		if snap.Tags == nil && len(tagFilters) > 0 {
			continue
		}
		if snap.ID == nil {
			continue
		}

		// Azure doesn't offer tag-filtering through the API so we have to manually
		// filter results. Require all filter keys to be present, with matching vals.
		for filterKey, filterVal := range tagFilters {
			if val, ok := (*snap.Tags)[filterKey]; !ok || val == nil || *val != filterVal {
				continue Snapshot
			}
		}

		ret = append(ret, *snap.Name)
	}

	return ret, nil
}

func (op *blockStorageAdapter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	fullDiskName := getFullDiskName(op.subscription, op.resourceGroup, volumeID)
	// snapshot names must be <= 80 characters long
	var snapshotName string
	suffix := "-" + uuid.NewV4().String()

	if len(volumeID) <= (80 - len(suffix)) {
		snapshotName = volumeID + suffix
	} else {
		snapshotName = volumeID[0:80-len(suffix)] + suffix
	}

	snap := disk.Snapshot{
		Name: &snapshotName,
		Properties: &disk.Properties{
			CreationData: &disk.CreationData{
				CreateOption:     disk.Copy,
				SourceResourceID: &fullDiskName,
			},
		},
		Tags:     &map[string]*string{},
		Location: &op.location,
	}

	for k, v := range tags {
		val := v
		(*snap.Tags)[k] = &val
	}

	ctx, cancel := context.WithTimeout(context.Background(), op.apiTimeout)
	defer cancel()

	_, errChan := op.snaps.CreateOrUpdate(op.resourceGroup, *snap.Name, snap, ctx.Done())

	err := <-errChan

	if err != nil {
		return "", errors.WithStack(err)
	}

	return snapshotName, nil
}

func (op *blockStorageAdapter) DeleteSnapshot(snapshotID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), op.apiTimeout)
	defer cancel()

	_, errChan := op.snaps.Delete(op.resourceGroup, snapshotID, ctx.Done())

	err := <-errChan

	return errors.WithStack(err)
}

func getFullDiskName(subscription string, resourceGroup string, diskName string) string {
	return fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Compute/disks/%v", subscription, resourceGroup, diskName)
}

func getFullSnapshotName(subscription string, resourceGroup string, snapshotName string) string {
	return fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Compute/snapshots/%v", subscription, resourceGroup, snapshotName)
}
