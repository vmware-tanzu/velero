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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	"github.com/pkg/errors"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/velero/pkg/cloudprovider"
	"github.com/heptio/velero/pkg/util/collections"
)

type blockStore struct {
	client *gophercloud.ServiceClient
	log    logrus.FieldLogger
}

func NewBlockStore(logger logrus.FieldLogger) cloudprovider.BlockStore {
	return &blockStore{log: logger}
}

func (b *blockStore) Init(config map[string]string) error {
	// Initialize the client from environment variables
	pc, err := authenticate()
	if err != nil {
		return errors.WithStack(err)
	}

	region := getRegion()

	client, err := openstack.NewBlockStorageV2(pc, gophercloud.EndpointOpts{
		Type:   "volumev2",
		Region: region,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	b.client = client

	return nil
}

func (b *blockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	// get the snapshot so we can apply its tags to the volume
	var vol *volumes.Volume
	opts := volumes.CreateOpts{
		SnapshotID:       snapshotID,
		VolumeType:       volumeType,
		AvailabilityZone: volumeAZ,
	}
	vol, err = volumes.Create(b.client, opts).Extract()
	if err == nil {
		volumeID = vol.ID
	}
	return volumeID, err
}

func (b *blockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	var (
		res *volumes.Volume
		err error
	)

	res, err = volumes.Get(b.client, volumeID).Extract()
	if err != nil {
		return "", nil, errors.WithStack(err)
	}
	return res.VolumeType, nil, nil
}

func (b *blockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	// snapshot names must adhere to RFC1035 and be 1-63 characters
	// long
	var snapshotName string
	suffix := "-" + uuid.NewV4().String()

	if len(volumeID) <= (63 - len(suffix)) {
		snapshotName = volumeID + suffix
	} else {
		snapshotName = volumeID[0:63-len(suffix)] + suffix
	}

	opts := &snapshots.CreateOpts{
		VolumeID: volumeID,
		Name:     snapshotName,
		Metadata: tags,
		Force:    true, //Force the snapshotting of inuse objects
	}

	snap, err := snapshots.Create(b.client, opts).Extract()
	if err != nil {
		return "", err
	}
	// There's little value in rewrapping these gophercloud types into yet another abstraction/type, instead just
	// return the gophercloud item
	return snap.Name, nil
}

func (b *blockStore) DeleteSnapshot(snapshotID string) error {
	err := snapshots.Delete(b.client, snapshotID).ExtractErr()
	if err != nil {
		return errors.WithStack(err)
	}
	return err
}

func (b *blockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "spec.cinder") {
		return "", nil
	}

	volumeID, err := collections.GetString(pv.UnstructuredContent(), "spec.cinder.volumeID")
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

func (b *blockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	cinder, err := collections.GetMap(pv.UnstructuredContent(), "spec.cinder")
	if err != nil {
		return nil, err
	}

	cinder["volumeID"] = volumeID

	return pv, nil
}
