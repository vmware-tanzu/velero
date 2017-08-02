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

package restorers

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
)

type persistentVolumeRestorer struct {
	snapshotService cloudprovider.SnapshotService
}

var _ ResourceRestorer = &persistentVolumeRestorer{}

func NewPersistentVolumeRestorer(snapshotService cloudprovider.SnapshotService) ResourceRestorer {
	return &persistentVolumeRestorer{
		snapshotService: snapshotService,
	}
}

func (sr *persistentVolumeRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (sr *persistentVolumeRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error) {
	if _, err := resetMetadataAndStatus(obj, false); err != nil {
		return nil, err
	}

	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, err
	}

	delete(spec, "claimRef")
	delete(spec, "storageClassName")

	if restore.Spec.RestorePVs {
		volumeID, err := sr.restoreVolume(obj.UnstructuredContent(), restore, backup)
		if err != nil {
			return nil, err
		}

		if err := setVolumeID(spec, volumeID); err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func (sr *persistentVolumeRestorer) Wait() bool {
	return true
}

func (sr *persistentVolumeRestorer) Ready(obj runtime.Unstructured) bool {
	phase, err := collections.GetString(obj.UnstructuredContent(), "status.phase")

	return err == nil && phase == "Available"
}

func setVolumeID(spec map[string]interface{}, volumeID string) error {
	if pvSource, found := spec["awsElasticBlockStore"]; found {
		pvSourceObj := pvSource.(map[string]interface{})
		pvSourceObj["volumeID"] = volumeID
		return nil
	} else if pvSource, found := spec["gcePersistentDisk"]; found {
		pvSourceObj := pvSource.(map[string]interface{})
		pvSourceObj["pdName"] = volumeID
		return nil
	} else if pvSource, found := spec["azureDisk"]; found {
		pvSourceObj := pvSource.(map[string]interface{})
		pvSourceObj["diskName"] = volumeID
		return nil
	}

	return errors.New("persistent volume source is not compatible")
}

func (sr *persistentVolumeRestorer) restoreVolume(item map[string]interface{}, restore *api.Restore, backup *api.Backup) (string, error) {
	pvName, err := collections.GetString(item, "metadata.name")
	if err != nil {
		return "", err
	}

	if backup.Status.VolumeBackups == nil {
		return "", fmt.Errorf("VolumeBackups map not found for persistent volume %s", pvName)
	}

	backupInfo, found := backup.Status.VolumeBackups[pvName]
	if !found {
		return "", fmt.Errorf("BackupInfo not found for PersistentVolume %s", pvName)
	}

	return sr.snapshotService.CreateVolumeFromSnapshot(backupInfo.SnapshotID, backupInfo.Type, backupInfo.Iops)
}
