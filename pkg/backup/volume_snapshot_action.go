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
	"fmt"
	"regexp"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/clock"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
)

// volumeSnapshotAction is a struct that knows how to take snapshots of PersistentVolumes
// that are backed by compatible cloud volumes.
type volumeSnapshotAction struct {
	snapshotService cloudprovider.SnapshotService
	clock           clock.Clock
}

var _ Action = &volumeSnapshotAction{}

func NewVolumeSnapshotAction(snapshotService cloudprovider.SnapshotService) Action {
	return &volumeSnapshotAction{
		snapshotService: snapshotService,
		clock:           clock.RealClock{},
	}
}

// Execute triggers a snapshot for the volume/disk underlying a PersistentVolume if the provided
// backup has volume snapshots enabled and the PV is of a compatible type. Also records cloud
// disk type and IOPS (if applicable) to be able to restore to current state later.
func (a *volumeSnapshotAction) Execute(volume map[string]interface{}, backup *api.Backup) error {
	backupName := fmt.Sprintf("%s/%s", backup.Namespace, backup.Name)
	if !backup.Spec.SnapshotVolumes {
		glog.V(2).Infof("Backup %q has volume snapshots disabled; skipping volume snapshot action.", backupName)
		return nil
	}

	metadata := volume["metadata"].(map[string]interface{})
	name := metadata["name"].(string)

	volumeID := getVolumeID(volume)
	if volumeID == "" {
		return fmt.Errorf("unable to determine volume ID for backup %q, PersistentVolume %q", backupName, name)
	}

	expiration := a.clock.Now().Add(backup.Spec.TTL.Duration)

	glog.Infof("Backup %q: snapshotting PersistenVolume %q, volume-id %q, expiration %v", backupName, name, volumeID, expiration)

	snapshotID, err := a.snapshotService.CreateSnapshot(volumeID)
	if err != nil {
		glog.V(4).Infof("error creating snapshot for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
		return err
	}

	volumeType, iops, err := a.snapshotService.GetVolumeInfo(volumeID)
	if err != nil {
		glog.V(4).Infof("error getting volume info for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
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

var ebsVolumeIDRegex = regexp.MustCompile("vol-.*")

func getVolumeID(pv map[string]interface{}) string {
	spec, err := collections.GetMap(pv, "spec")
	if err != nil {
		return ""
	}

	if aws, err := collections.GetMap(spec, "awsElasticBlockStore"); err == nil {
		volumeID, err := collections.GetString(aws, "volumeID")
		if err != nil {
			return ""
		}
		return ebsVolumeIDRegex.FindString(volumeID)
	}

	if gce, err := collections.GetMap(spec, "gcePersistentDisk"); err == nil {
		volumeID, err := collections.GetString(gce, "pdName")
		if err != nil {
			return ""
		}
		return volumeID
	}

	if gce, err := collections.GetMap(spec, "azureDisk"); err == nil {
		volumeID, err := collections.GetString(gce, "diskName")
		if err != nil {
			return ""
		}
		return volumeID
	}

	return ""
}
