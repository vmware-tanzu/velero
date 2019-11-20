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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupStorageLocation is a location where Velero stores backup objects.
type BackupStorageLocation struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec BackupStorageLocationSpec `json:"spec,omitempty"`

	// +optional
	Status BackupStorageLocationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupStorageLocationList is a list of BackupStorageLocations.
type BackupStorageLocationList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []BackupStorageLocation `json:"items"`
}

// StorageType represents the type of storage that a backup location uses.
// ObjectStorage must be non-nil, since it is currently the only supported StorageType.
type StorageType struct {
	ObjectStorage *ObjectStorageLocation `json:"objectStorage"`
}

// ObjectStorageLocation specifies the settings necessary to connect to a provider's object storage.
type ObjectStorageLocation struct {
	// Bucket is the bucket to use for object storage.
	Bucket string `json:"bucket"`

	// Prefix is the path inside a bucket to use for Velero storage. Optional.
	// +optional
	Prefix string `json:"prefix,omitempty"`
}

// BackupStorageLocationSpec defines the specification for a Velero BackupStorageLocation.
type BackupStorageLocationSpec struct {
	// Provider is the provider of the backup storage.
	Provider string `json:"provider"`

	// Config is for provider-specific configuration fields.
	// +optional
	Config map[string]string `json:"config,omitempty"`

	StorageType `json:",inline"`

	// AccessMode defines the permissions for the backup storage location.
	// +optional
	AccessMode BackupStorageLocationAccessMode `json:"accessMode,omitempty"`

	// BackupSyncPeriod defines how frequently to sync backup API objects from object storage. A value of 0 disables sync.
	// +optional
	// +nullable
	BackupSyncPeriod *metav1.Duration `json:"backupSyncPeriod,omitempty"`
}

// BackupStorageLocationPhase is the lifecyle phase of a Velero BackupStorageLocation.
// +kubebuilder:validation:Enum=Available;Unavailable
type BackupStorageLocationPhase string

const (
	// BackupStorageLocationPhaseAvailable means the location is available to read and write from.
	BackupStorageLocationPhaseAvailable BackupStorageLocationPhase = "Available"

	// BackupStorageLocationPhaseUnavailable means the location is unavailable to read and write from.
	BackupStorageLocationPhaseUnavailable BackupStorageLocationPhase = "Unavailable"
)

// BackupStorageLocationAccessMode represents the permissions for a BackupStorageLocation.
// +kubebuilder:validation:Enum=ReadOnly;ReadWrite
type BackupStorageLocationAccessMode string

const (
	// BackupStorageLocationAccessModeReadOnly represents read-only access to a BackupStorageLocation.
	BackupStorageLocationAccessModeReadOnly BackupStorageLocationAccessMode = "ReadOnly"

	// BackupStorageLocationAccessModeReadWrite represents read and write access to a BackupStorageLocation.
	BackupStorageLocationAccessModeReadWrite BackupStorageLocationAccessMode = "ReadWrite"
)

// TODO(2.0): remove the AccessMode field from BackupStorageLocationStatus.
// TODO(2.0): remove the LastSyncedRevision field from BackupStorageLocationStatus.

// BackupStorageLocationStatus describes the current status of a Velero BackupStorageLocation.
type BackupStorageLocationStatus struct {
	// Phase is the current state of the BackupStorageLocation.
	// +optional
	Phase BackupStorageLocationPhase `json:"phase,omitempty"`

	// LastSyncedTime is the last time the contents of the location were synced into
	// the cluster.
	// +optional
	// +nullable
	LastSyncedTime *metav1.Time `json:"lastSyncedTime,omitempty"`

	// LastSyncedRevision is the value of the `metadata/revision` file in the backup
	// storage location the last time the BSL's contents were synced into the cluster.
	//
	// Deprecated: this field is no longer updated or used for detecting changes to
	// the location's contents and will be removed entirely in v2.0.
	// +optional
	LastSyncedRevision types.UID `json:"lastSyncedRevision,omitempty"`

	// AccessMode is an unused field.
	//
	// Deprecated: there is now an AccessMode field on the Spec and this field
	// will be removed entirely as of v2.0.
	// +optional
	AccessMode BackupStorageLocationAccessMode `json:"accessMode,omitempty"`
}
