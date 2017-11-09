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
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	"github.com/satori/uuid"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type blockStorageAdapter struct {
	cinderClient *gophercloud.ServiceClient
}

var _ cloudprovider.BlockStorageAdapter = &blockStorageAdapter{}

func NewBlockStorageAdapter(region string) (cloudprovider.BlockStorageAdapter, error) {
	provider, err := getProvider()
	if err != nil {
		return nil, err
	}

	cinderClient, err := openstack.NewBlockStorageV2(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not set up Cinder client")
	}

	return &blockStorageAdapter{cinderClient}, nil
}

func (op *blockStorageAdapter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	createOpts := volumes.CreateOpts{
		Name:       fmt.Sprintf("ark-backup-%s", uuid.NewV4().String()),
		SnapshotID: snapshotID,
		VolumeType: volumeType,
	}
	vol, err := volumes.Create(op.cinderClient, createOpts).Extract()
	if err != nil {
		return "", err
	}
	return vol.Name, nil
}

func (op *blockStorageAdapter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	vol, err := volumes.Get(op.cinderClient, volumeID).Extract()
	return vol.Name, nil, err
}

func (op *blockStorageAdapter) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	vol, err := volumes.Get(op.cinderClient, volumeID).Extract()
	return strings.ToLower(vol.Status) == "available", err
}

func (op *blockStorageAdapter) ListSnapshots(tagFilters map[string]string) ([]string, error) {
	snapshotNames := []string{}

	allPages, err := snapshots.List(op.cinderClient, snapshots.ListOpts{}).AllPages()
	if err != nil {
		return snapshotNames, err
	}

	allSnapshots, err := snapshots.ExtractSnapshots(allPages)
	if err != nil {
		return snapshotNames, err
	}

	for _, snapshot := range allSnapshots {
		if snapshotHasAllTags(snapshot, tagFilters) {
			snapshotNames = append(snapshotNames, snapshot.Name)
		}
	}

	return snapshotNames, nil
}

func (op *blockStorageAdapter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	createOpts := snapshots.CreateOpts{
		VolumeID: volumeID,
		Metadata: tags,
	}
	snapshot, err := snapshots.Create(op.cinderClient, createOpts).Extract()
	return snapshot.Name, err
}

func (op *blockStorageAdapter) DeleteSnapshot(snapshotID string) error {
	return snapshots.Delete(op.cinderClient, snapshotID).ExtractErr()
}

func snapshotHasAllTags(snapshot snapshots.Snapshot, tagFilters map[string]string) bool {
	for k, v := range tagFilters {
		if sv, ok := snapshot.Metadata[k]; !ok || v != sv {
			return false
		}
	}
	return true
}
