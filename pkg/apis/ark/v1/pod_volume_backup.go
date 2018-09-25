/*
Copyright 2018 the Heptio Ark contributors.

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

// PodVolumeBackupSpec is the specification for a PodVolumeBackup.
type PodVolumeBackupSpec struct {
	// Node is the name of the node that the Pod is running on.
	Node string `json:"node"`

	// Pod is a reference to the pod containing the volume to be backed up.
	Pod corev1api.ObjectReference `json:"pod"`

	// Volume is the name of the volume within the Pod to be backed
	// up.
	Volume string `json:"volume"`

	// BackupStorageLocation is the name of the backup storage location
	// where the restic repository is stored.
	BackupStorageLocation string `json:"backupStorageLocation"`

	// RepoIdentifier is the restic repository identifier.
	RepoIdentifier string `json:"repoIdentifier"`

	// Tags are a map of key-value pairs that should be applied to the
	// volume backup as tags.
	Tags map[string]string `json:"tags"`
}

// PodVolumeBackupPhase represents the lifecycle phase of a PodVolumeBackup.
type PodVolumeBackupPhase string

const (
	PodVolumeBackupPhaseNew        PodVolumeBackupPhase = "New"
	PodVolumeBackupPhaseInProgress PodVolumeBackupPhase = "InProgress"
	PodVolumeBackupPhaseCompleted  PodVolumeBackupPhase = "Completed"
	PodVolumeBackupPhaseFailed     PodVolumeBackupPhase = "Failed"
)

// PodVolumeBackupStatus is the current status of a PodVolumeBackup.
type PodVolumeBackupStatus struct {
	// Phase is the current state of the PodVolumeBackup.
	Phase PodVolumeBackupPhase `json:"phase"`

	// Path is the full path within the controller pod being backed up.
	Path string `json:"path"`

	// SnapshotID is the identifier for the snapshot of the pod volume.
	SnapshotID string `json:"snapshotID"`

	// Message is a message about the pod volume backup's status.
	Message string `json:"message"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodVolumeBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   PodVolumeBackupSpec   `json:"spec"`
	Status PodVolumeBackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodVolumeBackupList is a list of PodVolumeBackups.
type PodVolumeBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PodVolumeBackup `json:"items"`
}
