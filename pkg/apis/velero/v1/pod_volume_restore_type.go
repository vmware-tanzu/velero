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
	// where the backup repository is stored.
	BackupStorageLocation string `json:"backupStorageLocation"`

	// RepoIdentifier is the backup repository identifier.
	RepoIdentifier string `json:"repoIdentifier"`

	// UploaderType is the type of the uploader to handle the data transfer.
	// +kubebuilder:validation:Enum=kopia;restic;""
	// +optional
	UploaderType string `json:"uploaderType"`

	// SnapshotID is the ID of the volume snapshot to be restored.
	SnapshotID string `json:"snapshotID"`

	// SourceNamespace is the original namespace for namaspace mapping.
	SourceNamespace string `json:"sourceNamespace"`
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

// TODO(2.0) After converting all resources to use the runtime-controller client, the genclient and k8s:deepcopy markers will no longer be needed and should be removed.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".spec.pod.namespace",description="Namespace of the pod containing the volume to be restored"
// +kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".spec.pod.name",description="Name of the pod containing the volume to be restored"
// +kubebuilder:printcolumn:name="Uploader Type",type="string",JSONPath=".spec.uploaderType",description="The type of the uploader to handle data transfer"
// +kubebuilder:printcolumn:name="Volume",type="string",JSONPath=".spec.volume",description="Name of the volume to be restored"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Pod Volume Restore status such as New/InProgress"
// +kubebuilder:printcolumn:name="TotalBytes",type="integer",format="int64",JSONPath=".status.progress.totalBytes",description="Pod Volume Restore status such as New/InProgress"
// +kubebuilder:printcolumn:name="BytesDone",type="integer",format="int64",JSONPath=".status.progress.bytesDone",description="Pod Volume Restore status such as New/InProgress"
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
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// PodVolumeRestoreList is a list of PodVolumeRestores.
type PodVolumeRestoreList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PodVolumeRestore `json:"items"`
}
