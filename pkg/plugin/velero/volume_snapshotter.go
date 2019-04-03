/*
Copyright 2017, 2019 the Velero contributors.

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

package velero

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// VolumeSnapshotter defines the operations needed by Velero to
// take snapshots of persistent volumes during backup, and to restore
// persistent volumes from snapshots during restore.
type VolumeSnapshotter interface {
	// Init prepares the VolumeSnapshotter for usage using the provided map of
	// configuration key-value pairs. It returns an error if the VolumeSnapshotter
	// cannot be initialized from the provided config.
	Init(config map[string]string) error

	// CreateVolumeFromSnapshot creates a new volume in the specified
	// availability zone, initialized from the provided snapshot,
	// and with the specified type and IOPS (if using provisioned IOPS).
	CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error)

	// GetVolumeID returns the cloud provider specific identifier for the PersistentVolume.
	GetVolumeID(pv runtime.Unstructured) (string, error)

	// SetVolumeID sets the cloud provider specific identifier for the PersistentVolume.
	SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error)

	// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for
	// the specified volume in the given availability zone.
	GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error)

	// CreateSnapshot creates a snapshot of the specified volume, and applies the provided
	// set of tags to the snapshot.
	CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error)

	// DeleteSnapshot deletes the specified volume snapshot.
	DeleteSnapshot(snapshotID string) error
}
