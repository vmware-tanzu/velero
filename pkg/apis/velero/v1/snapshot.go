/*
Copyright 2020 the Velero contributors.

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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type SnapshotSpec struct {
	// BackupName is the name of the Velero backup this snapshot
	// is associated with.
	BackupName string `json:"backupName"`

	// BackupUID is the UID of the Velero backup this snapshot
	// is associated with.
	// +optional
	BackupUID string `json:"backupUID"`

	// Location is the name of the VolumeSnapshotLocation where this snapshot is stored.
	// +optional
	Location string `json:"location"`

	// PersistentVolumeName is the Kubernetes name for the volume.
	// +optional
	PersistentVolumeName string `json:"persistentVolumeName"`

	// ProviderVolumeID is the provider's ID for the volume.
	// +optional
	ProviderVolumeID string `json:"providerVolumeID"`

	// VolumeType is the type of the disk/volume in the cloud provider
	// API.
	// +optional
	VolumeType string `json:"volumeType"`

	// VolumeAZ is the where the volume is provisioned
	// in the cloud provider.
	// +optional
	VolumeAZ string `json:"volumeAZ,omitempty"`

	// VolumeIOPS is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	// +optional
	VolumeIOPS *int64 `json:"volumeIOPS,omitempty"`
}

type SnapshotStatus struct {
	// ProviderSnapshotID is the ID of the snapshot taken in the cloud
	// provider API of this volume.
	// +optional
	ProviderSnapshotID string `json:"providerSnapshotID,omitempty"`

	// Phase is the current state of the VolumeSnapshot.
	// +optional
	Phase SnapshotPhase `json:"phase,omitempty"`
}

// SnapshotPhase is the lifecyle phase of a Velero volume snapshot.
type SnapshotPhase string

const (
	// SnapshotPhaseNew means the volume snapshot has been created but not
	// yet processed by the VolumeSnapshotController.
	SnapshotPhaseNew SnapshotPhase = "New"

	// SnapshotPhaseCompleted means the volume snapshot was successfully created and can be restored from..
	SnapshotPhaseCompleted SnapshotPhase = "Completed"

	// SnapshotPhaseFailed means the volume snapshot was unable to execute.
	SnapshotPhaseFailed SnapshotPhase = "Failed"

	// SnapshotPhaseInProgress means the volume snapshot created, but isn't ready yet.
	SnapshotPhaseInProgress SnapshotPhase = "InProgress"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Snapshot stores information about a persistent volume snapshot taken as
// part of a Velero backup.
type Snapshot struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SnapshotSpec `json:"spec"`

	// +optional
	Status SnapshotStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SnapshotList is a list of VolumeSnapshots.
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Snapshot `json:"items"`
}
