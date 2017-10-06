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
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
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

func (sr *persistentVolumeRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	if _, err := resetMetadataAndStatus(obj, false); err != nil {
		return nil, nil, err
	}

	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, nil, err
	}

	delete(spec, "claimRef")
	delete(spec, "storageClassName")

	pvName, err := collections.GetString(obj.UnstructuredContent(), "metadata.name")
	if err != nil {
		return nil, nil, err
	}

	// if it's an unsupported volume type for snapshot restores, we're done
	if sourceType, _ := kubeutil.GetPVSource(spec); sourceType == "" {
		return obj, nil, nil
	}

	restoreFromSnapshot := false

	if restore.Spec.RestorePVs != nil && *restore.Spec.RestorePVs {
		// when RestorePVs = yes, it's an error if we don't have a snapshot service
		if sr.snapshotService == nil {
			return nil, nil, errors.New("PV restorer is not configured for PV snapshot restores")
		}

		// if there are no snapshots in the backup, return without error
		if backup.Status.VolumeBackups == nil {
			return obj, nil, nil
		}

		// if there are snapshots, and this is a supported PV type, but there's no
		// snapshot for this PV, it's an error
		if backup.Status.VolumeBackups[pvName] == nil {
			return nil, nil, errors.Errorf("no snapshot found to restore volume %s from", pvName)
		}

		restoreFromSnapshot = true
	}
	if restore.Spec.RestorePVs == nil && sr.snapshotService != nil {
		// when RestorePVs = Auto, don't error if the backup doesn't have snapshots
		if backup.Status.VolumeBackups == nil || backup.Status.VolumeBackups[pvName] == nil {
			return obj, nil, nil
		}

		restoreFromSnapshot = true
	}

	if restoreFromSnapshot {
		backupInfo := backup.Status.VolumeBackups[pvName]

		volumeID, err := sr.snapshotService.CreateVolumeFromSnapshot(backupInfo.SnapshotID, backupInfo.Type, backupInfo.AvailabilityZone, backupInfo.Iops)
		if err != nil {
			return nil, nil, err
		}

		if err := kubeutil.SetVolumeID(spec, volumeID); err != nil {
			return nil, nil, err
		}
	}

	var warning error

	if sr.snapshotService == nil && len(backup.Status.VolumeBackups) > 0 {
		warning = errors.New("unable to restore PV snapshots: Ark server is not configured with a PersistentVolumeProvider")
	}

	return obj, warning, nil
}

func (sr *persistentVolumeRestorer) Wait() bool {
	return true
}

func (sr *persistentVolumeRestorer) Ready(obj runtime.Unstructured) bool {
	phase, err := collections.GetString(obj.UnstructuredContent(), "status.phase")

	return err == nil && phase == "Available"
}
