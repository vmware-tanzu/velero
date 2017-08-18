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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// BackupSpec defines the specification for an Ark backup.
type BackupSpec struct {
	// IncludedNamespaces is a slice of namespace names to include objects
	// from. If empty, all namespaces are included.
	IncludedNamespaces []string `json:"includedNamespaces"`

	// ExcludedNamespaces contains a list of namespaces that are not
	// included in the backup.
	ExcludedNamespaces []string `json:"excludedNamespaces"`

	// IncludedResources is a slice of resource names to include
	// in the backup. If empty, all resources are included.
	IncludedResources []string `json:"includedResources"`

	// ExcludedResources is a slice of resource names that are not
	// included in the backup.
	ExcludedResources []string `json:"excludedResources"`

	// LabelSelector is a metav1.LabelSelector to filter with
	// when adding individual objects to the backup. If empty
	// or nil, all objects are included. Optional.
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`

	// SnapshotVolumes specifies whether to take cloud snapshots
	// of any PV's referenced in the set of objects included
	// in the Backup.
	SnapshotVolumes *bool `json:"snapshotVolumes"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	TTL metav1.Duration `json:"ttl"`
}

// BackupPhase is a string representation of the lifecycle phase
// of an Ark backup.
type BackupPhase string

const (
	// BackupPhaseNew means the backup has been created but not
	// yet processed by the BackupController.
	BackupPhaseNew BackupPhase = "New"

	// BackupPhaseFailedValidation means the backup has failed
	// the controller's validations and therefore will not run.
	BackupPhaseFailedValidation BackupPhase = "FailedValidation"

	// BackupPhaseInProgress means the backup is currently executing.
	BackupPhaseInProgress BackupPhase = "InProgress"

	// BackupPhaseCompleted means the backup has run successfully without
	// errors.
	BackupPhaseCompleted BackupPhase = "Completed"

	// BackupPhaseFailed mean the backup ran but encountered an error that
	// prevented it from completing successfully.
	BackupPhaseFailed BackupPhase = "Failed"
)

// BackupStatus captures the current status of an Ark backup.
type BackupStatus struct {
	// Version is the backup format version.
	Version int `json:"version"`

	// Expiration is when this Backup is eligible for garbage-collection.
	Expiration metav1.Time `json:"expiration"`

	// Phase is the current state of the Backup.
	Phase BackupPhase `json:"phase"`

	// VolumeBackups is a map of PersistentVolume names to
	// information about the backed-up volume in the cloud
	// provider API.
	VolumeBackups map[string]*VolumeBackupInfo `json:"volumeBackups"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable).
	ValidationErrors []string `json:"validationErrors"`
}

// VolumeBackupInfo captures the required information about
// a PersistentVolume at backup time to be able to restore
// it later.
type VolumeBackupInfo struct {
	// SnapshotID is the ID of the snapshot taken in the cloud
	// provider API of this volume.
	SnapshotID string `json:"snapshotID"`

	// Type is the type of the disk/volume in the cloud provider
	// API.
	Type string `json:"type"`

	// Iops is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	Iops *int64 `json:"iops,omitempty"`
}

// +genclient=true

// Backup is an Ark resource that respresents the capture of Kubernetes
// cluster state at a point in time (API objects and associated volume state).
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BackupSpec   `json:"spec"`
	Status BackupStatus `json:"status,omitempty"`
}

// BackupList is a list of Backups.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Backup `json:"items"`
}
