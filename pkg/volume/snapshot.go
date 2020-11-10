/*
Copyright 2018 the Velero contributors.

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

package volume

// Snapshot stores information about a persistent volume snapshot taken as
// part of a Velero backup.
type Snapshot struct {
	Spec SnapshotSpec `json:"spec"`

	Status SnapshotStatus `json:"status"`
}

type SnapshotSpec struct {
	// BackupName is the name of the Velero backup this snapshot
	// is associated with.
	BackupName string `json:"backupName"`

	// BackupUID is the UID of the Velero backup this snapshot
	// is associated with.
	BackupUID string `json:"backupUID"`

	// Location is the name of the VolumeSnapshotLocation where this snapshot is stored.
	Location string `json:"location"`

	// PersistentVolumeName is the Kubernetes name for the volume.
	PersistentVolumeName string `json:persistentVolumeName`

	// ProviderVolumeID is the provider's ID for the volume.
	ProviderVolumeID string `json:"providerVolumeID"`

	// VolumeType is the type of the disk/volume in the cloud provider
	// API.
	VolumeType string `json:"volumeType"`

	// VolumeAZ is the where the volume is provisioned
	// in the cloud provider.
	VolumeAZ string `json:"volumeAZ,omitempty"`

	// VolumeIOPS is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	VolumeIOPS *int64 `json:"volumeIOPS,omitempty"`
}

type SnapshotStatus struct {
	// ProviderSnapshotID is the ID of the snapshot taken in the cloud
	// provider API of this volume.
	ProviderSnapshotID string `json:"providerSnapshotID,omitempty"`

	// Phase is the current state of the VolumeSnapshot.
	Phase SnapshotPhase `json:"phase,omitempty"`
}

// SnapshotPhase is the lifecycle phase of a Velero volume snapshot.
type SnapshotPhase string

const (
	// SnapshotPhaseNew means the volume snapshot has been created but not
	// yet processed by the VolumeSnapshotController.
	SnapshotPhaseNew SnapshotPhase = "New"

	// SnapshotPhaseCompleted means the volume snapshot was successfully created and can be restored from..
	SnapshotPhaseCompleted SnapshotPhase = "Completed"

	// SnapshotPhaseFailed means the volume snapshot was unable to execute.
	SnapshotPhaseFailed SnapshotPhase = "Failed"
)
