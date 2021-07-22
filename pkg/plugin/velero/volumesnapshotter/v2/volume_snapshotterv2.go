/*
Copyright 2021 the Velero contributors.

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

package v2

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/volumesnapshotter/v1"

	"context"
)

type VolumeSnapshotter interface {
	v1.VolumeSnapshotter

	// InitV2 prepares the VolumeSnapshotter for usage using the provided map of
	// configuration key-value pairs. It returns an error if the VolumeSnapshotter
	// cannot be initialized from the provided config.
	InitV2(ctx context.Context, config map[string]string) error

	// CreateVolumeFromSnapshotV2 creates a new volume in the specified
	// availability zone, initialized from the provided snapshot,
	// and with the specified type and IOPS (if using provisioned IOPS).
	CreateVolumeFromSnapshotV2(ctx context.Context,
		snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error)

	// GetVolumeIDV2 returns the cloud provider specific identifier for the PersistentVolume.
	GetVolumeIDV2(ctx context.Context, pv runtime.Unstructured) (string, error)

	// SetVolumeIDV2 sets the cloud provider specific identifier for the PersistentVolume.
	SetVolumeIDV2(ctx context.Context,
		pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error)

	// GetVolumeInfoV2 returns the type and IOPS (if using provisioned IOPS) for
	// the specified volume in the given availability zone.
	GetVolumeInfoV2(ctx context.Context, volumeID, volumeAZ string) (string, *int64, error)

	// CreateSnapshotV2 creates a snapshot of the specified volume, and applies the provided
	// set of tags to the snapshot.
	CreateSnapshotV2(ctx context.Context,
		volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error)

	// DeleteSnapshotV2 deletes the specified volume snapshot.
	DeleteSnapshotV2(ctx context.Context, snapshotID string) error
}
