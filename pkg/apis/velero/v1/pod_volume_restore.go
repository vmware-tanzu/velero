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

package v1

import (
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodVolumeRestoreSpec is the specification for a PodVolumeRestore.
type PodVolumeRestoreSpec struct {
	// Pod is a reference to the pod containing the volume to be restored.
	Pod corev1api.ObjectReference `json:"pod"`

	// Volume is the name of the volume within the Pod to be restored.
	Volume string `json:"volume"`

	// BackupStorageLocation is the name of the backup storage location
	// where the restic repository is stored.
	BackupStorageLocation string `json:"backupStorageLocation"`

	// RepoIdentifier is the restic repository identifier.
	RepoIdentifier string `json:"repoIdentifier"`

	// SnapshotID is the ID of the volume snapshot to be restored.
	SnapshotID string `json:"snapshotID"`
}

// PodVolumeRestorePhase represents the lifecycle phase of a PodVolumeRestore.
// +kubebuilder:validation:Enum=New;InProgress;Completed;Failed
type PodVolumeRestorePhase string

const (
	PodVolumeRestorePhaseNew        PodVolumeRestorePhase = "New"
	PodVolumeRestorePhaseInProgress PodVolumeRestorePhase = "InProgress"
	PodVolumeRestorePhaseCompleted  PodVolumeRestorePhase = "Completed"
	PodVolumeRestorePhaseFailed     PodVolumeRestorePhase = "Failed"
)

// PodVolumeRestoreStatus is the current status of a PodVolumeRestore.
type PodVolumeRestoreStatus struct {
	// Phase is the current state of the PodVolumeRestore.
	// +optional
	Phase PodVolumeRestorePhase `json:"phase,omitempty"`

	// Message is a message about the pod volume restore's status.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTimestamp records the time a restore was started.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a restore was completed.
	// Completion time is recorded even on failed restores.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Progress holds the total number of bytes of the snapshot and the current
	// number of restored bytes. This can be used to display progress information
	// about the restore operation.
	// +optional
	Progress PodVolumeOperationProgress `json:"progress,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Restore status such as InProgress/Completed"
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".spec.pod.namespace",description="Namespace of pod containing the volume to be restored"
// +kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".spec.pod.name",description="Name of pod containing the volume to be restored"
// +kubebuilder:printcolumn:name="Volume",type="string",JSONPath=".spec.volume",description="Name of volume to restore"
// +kubebuilder:printcolumn:name="Bytes Done",type="string",JSONPath=".status.progress.bytesDone",description="Number of bytes restored"
// +kubebuilder:printcolumn:name="Total Bytes",type="string",JSONPath=".status.progress.totalBytes",description="Total number of bytes to be restored"
// +kubebuilder:printcolumn:name="Started",type="date",JSONPath=".status.startTimestamp",description="Time when the restore operation was started"
// +kubebuilder:printcolumn:name="Completed",type="date",JSONPath=".status.completionTimestamp",description="Time when the restore operation was completed"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type PodVolumeRestore struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PodVolumeRestoreSpec `json:"spec,omitempty"`

	// +optional
	Status PodVolumeRestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodVolumeRestoreList is a list of PodVolumeRestores.
type PodVolumeRestoreList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PodVolumeRestore `json:"items"`
}
