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

package backup

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/util/clock"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
)

// volumeSnapshotAction is a struct that knows how to take snapshots of PersistentVolumes
// that are backed by compatible cloud volumes.
type volumeSnapshotAction struct {
	snapshotService cloudprovider.SnapshotService
	clock           clock.Clock
}

var _ Action = &volumeSnapshotAction{}

func NewVolumeSnapshotAction(snapshotService cloudprovider.SnapshotService) (Action, error) {
	if snapshotService == nil {
		return nil, errors.New("snapshotService cannot be nil")
	}

	return &volumeSnapshotAction{
		snapshotService: snapshotService,
		clock:           clock.RealClock{},
	}, nil
}

// Execute triggers a snapshot for the volume/disk underlying a PersistentVolume if the provided
// backup has volume snapshots enabled and the PV is of a compatible type. Also records cloud
// disk type and IOPS (if applicable) to be able to restore to current state later.
func (a *volumeSnapshotAction) Execute(ctx ActionContext, volume map[string]interface{}, backup *api.Backup) error {
	backupName := fmt.Sprintf("%s/%s", backup.Namespace, backup.Name)
	if backup.Spec.SnapshotVolumes != nil && !*backup.Spec.SnapshotVolumes {
		ctx.log("Backup %q has volume snapshots disabled; skipping volume snapshot action.", backupName)
		return nil
	}

	metadata := volume["metadata"].(map[string]interface{})
	name := metadata["name"].(string)

	volumeID, err := kubeutil.GetVolumeID(volume)
	// non-nil error means it's a supported PV source but volume ID can't be found
	if err != nil {
		return fmt.Errorf("error getting volume ID for backup %q, PersistentVolume %q: %v", backupName, name, err)
	}
	// no volumeID / nil error means unsupported PV source
	if volumeID == "" {
		ctx.log("Backup %q: PersistentVolume %q is not a supported volume type for snapshots, skipping.", backupName, name)
		return nil
	}

	expiration := a.clock.Now().Add(backup.Spec.TTL.Duration)

	ctx.log("Backup %q: snapshotting PersistentVolume %q, volume-id %q, expiration %v", backupName, name, volumeID, expiration)

	snapshotID, err := a.snapshotService.CreateSnapshot(volumeID)
	if err != nil {
		ctx.log("error creating snapshot for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
		return err
	}

	volumeType, iops, err := a.snapshotService.GetVolumeInfo(volumeID)
	if err != nil {
		ctx.log("error getting volume info for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
		return err
	}

	if backup.Status.VolumeBackups == nil {
		backup.Status.VolumeBackups = make(map[string]*api.VolumeBackupInfo)
	}

	backup.Status.VolumeBackups[name] = &api.VolumeBackupInfo{
		SnapshotID: snapshotID,
		Type:       volumeType,
		Iops:       iops,
	}

	return nil
}
