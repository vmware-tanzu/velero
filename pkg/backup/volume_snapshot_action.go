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
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
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
func (a *volumeSnapshotAction) Execute(log *logrus.Entry, item runtime.Unstructured, backup *api.Backup) ([]ResourceIdentifier, error) {
	var noAdditionalItems []ResourceIdentifier

	log.Info("Executing volumeSnapshotAction")

	if backup.Spec.SnapshotVolumes != nil && !*backup.Spec.SnapshotVolumes {
		log.Info("Backup has volume snapshots disabled; skipping volume snapshot action.")
		return noAdditionalItems, nil
	}

	metadata, err := meta.Accessor(item)
	if err != nil {
		return noAdditionalItems, errors.WithStack(err)
	}

	name := metadata.GetName()
	var pvFailureDomainZone string
	labels := metadata.GetLabels()

	if labels[zoneLabel] != "" {
		pvFailureDomainZone = labels[zoneLabel]
	} else {
		log.Infof("label %q is not present on PersistentVolume", zoneLabel)
	}

	volumeID, err := kubeutil.GetVolumeID(item.UnstructuredContent())
	// non-nil error means it's a supported PV source but volume ID can't be found
	if err != nil {
		return noAdditionalItems, errors.Wrapf(err, "error getting volume ID for PersistentVolume")
	}
	// no volumeID / nil error means unsupported PV source
	if volumeID == "" {
		log.Info("PersistentVolume is not a supported volume type for snapshots, skipping.")
		return noAdditionalItems, nil
	}

	log = log.WithField("volumeID", volumeID)

	log.Info("Snapshotting PersistentVolume")
	snapshotID, err := a.snapshotService.CreateSnapshot(volumeID, pvFailureDomainZone)
	if err != nil {
		// log+error on purpose - log goes to the per-backup log file, error goes to the backup
		log.WithError(err).Error("error creating snapshot")
		return noAdditionalItems, errors.WithMessage(err, "error creating snapshot")
	}

	volumeType, iops, err := a.snapshotService.GetVolumeInfo(volumeID, pvFailureDomainZone)
	if err != nil {
		log.WithError(err).Error("error getting volume info")
		return noAdditionalItems, errors.WithMessage(err, "error getting volume info")
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

	return noAdditionalItems, nil
}
