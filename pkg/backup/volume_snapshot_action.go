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
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/util/clock"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
)

// zoneLabel is the label that stores availability-zone info
// on PVs
const zoneLabel = "failure-domain.beta.kubernetes.io/zone"

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
func (a *volumeSnapshotAction) Execute(ctx *backupContext, volume map[string]interface{}, backupper itemBackupper) error {
	var (
		backup     = ctx.backup
		backupName = kubeutil.NamespaceAndName(backup)
	)

	if backup.Spec.SnapshotVolumes != nil && !*backup.Spec.SnapshotVolumes {
		ctx.infof("Backup %q has volume snapshots disabled; skipping volume snapshot action.", backupName)
		return nil
	}

	metadata := volume["metadata"].(map[string]interface{})
	name := metadata["name"].(string)
	var pvFailureDomainZone string

	if labelsMap, err := collections.GetMap(metadata, "labels"); err != nil {
		ctx.infof("error getting labels on PersistentVolume %q for backup %q: %v", name, backupName, err)
	} else {
		if labelsMap[zoneLabel] != nil {
			pvFailureDomainZone = labelsMap[zoneLabel].(string)
		} else {
			ctx.infof("label %q is not present on PersistentVolume %q for backup %q.", zoneLabel, name, backupName)
		}
	}

	volumeID, err := kubeutil.GetVolumeID(volume)
	// non-nil error means it's a supported PV source but volume ID can't be found
	if err != nil {
		return errors.Wrapf(err, "error getting volume ID for backup %q, PersistentVolume %q", backupName, name)
	}
	// no volumeID / nil error means unsupported PV source
	if volumeID == "" {
		ctx.infof("Backup %q: PersistentVolume %q is not a supported volume type for snapshots, skipping.", backupName, name)
		return nil
	}

	ctx.infof("Backup %q: snapshotting PersistentVolume %q, volume-id %q", backupName, name, volumeID)
	snapshotID, err := a.snapshotService.CreateSnapshot(volumeID, pvFailureDomainZone)
	if err != nil {
		ctx.infof("error creating snapshot for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
		return err
	}

	volumeType, iops, err := a.snapshotService.GetVolumeInfo(volumeID, pvFailureDomainZone)
	if err != nil {
		ctx.infof("error getting volume info for backup %q, volume %q, volume-id %q: %v", backupName, name, volumeID, err)
		return err
	}

	if backup.Status.VolumeBackups == nil {
		backup.Status.VolumeBackups = make(map[string]*api.VolumeBackupInfo)
	}

	backup.Status.VolumeBackups[name] = &api.VolumeBackupInfo{
		SnapshotID:       snapshotID,
		Type:             volumeType,
		Iops:             iops,
		AvailabilityZone: pvFailureDomainZone,
	}

	return nil
}
