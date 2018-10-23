/*
Copyright 2017 the Heptio Ark contributors.

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
)

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
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	TTL metav1.Duration `json:"ttl"`

	// IncludeClusterResources specifies whether cluster-scoped resources
	// should be included for consideration in the backup.
	IncludeClusterResources *bool `json:"includeClusterResources"`

	// Hooks represent custom behaviors that should be executed at different phases of the backup.
	Hooks BackupHooks `json:"hooks"`

	// StorageLocation is a string containing the name of a BackupStorageLocation where the backup should be stored.
	StorageLocation string `json:"storageLocation"`

	// VolumeSnapshotLocations is a list containing names of VolumeSnapshotLocations associated with this backup.
	VolumeSnapshotLocations []string `json:"volumeSnapshotLocations"`
}

// BackupHooks contains custom behaviors that should be executed at different phases of the backup.
type BackupHooks struct {
	// Resources are hooks that should be executed when backing up individual instances of a resource.
	Resources []BackupResourceHookSpec `json:"resources"`
}

// BackupResourceHookSpec defines one or more BackupResourceHooks that should be executed based on
// the rules defined for namespaces, resources, and label selector.
type BackupResourceHookSpec struct {
	// Name is the name of this hook.
	Name string `json:"name"`
	// IncludedNamespaces specifies the namespaces to which this hook spec applies. If empty, it applies
	// to all namespaces.
	IncludedNamespaces []string `json:"includedNamespaces"`
	// ExcludedNamespaces specifies the namespaces to which this hook spec does not apply.
	ExcludedNamespaces []string `json:"excludedNamespaces"`
	// IncludedResources specifies the resources to which this hook spec applies. If empty, it applies
	// to all resources.
	IncludedResources []string `json:"includedResources"`
	// ExcludedResources specifies the resources to which this hook spec does not apply.
	ExcludedResources []string `json:"excludedResources"`
	// LabelSelector, if specified, filters the resources to which this hook spec applies.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// Hooks is a list of BackupResourceHooks to execute. DEPRECATED. Replaced by PreHooks.
	Hooks []BackupResourceHook `json:"hooks"`
	// PreHooks is a list of BackupResourceHooks to execute prior to storing the item in the backup.
	// These are executed before any "additional items" from item actions are processed.
	PreHooks []BackupResourceHook `json:"pre,omitempty"`
	// PostHooks is a list of BackupResourceHooks to execute after storing the item in the backup.
	// These are executed after all "additional items" from item actions are processed.
	PostHooks []BackupResourceHook `json:"post,omitempty"`
}

// BackupResourceHook defines a hook for a resource.
type BackupResourceHook struct {
	// Exec defines an exec hook.
	Exec *ExecHook `json:"exec"`
}

// ExecHook is a hook that uses the pod exec API to execute a command in a container in a pod.
type ExecHook struct {
	// Container is the container in the pod where the command should be executed. If not specified,
	// the pod's first container is used.
	Container string `json:"container"`
	// Command is the command and arguments to execute.
	Command []string `json:"command"`
	// OnError specifies how Ark should behave if it encounters an error executing this hook.
	OnError HookErrorMode `json:"onError"`
	// Timeout defines the maximum amount of time Ark should wait for the hook to complete before
	// considering the execution a failure.
	Timeout metav1.Duration `json:"timeout"`
}

// HookErrorMode defines how Ark should treat an error from a hook.
type HookErrorMode string

const (
	// HookErrorModeContinue means that an error from a hook is acceptable, and the backup can
	// proceed.
	HookErrorModeContinue HookErrorMode = "Continue"
	// HookErrorModeFail means that an error from a hook is problematic, and the backup should be in
	// error.
	HookErrorModeFail HookErrorMode = "Fail"
)

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

	// BackupPhaseFailed means the backup ran but encountered an error that
	// prevented it from completing successfully.
	BackupPhaseFailed BackupPhase = "Failed"

	// BackupPhaseDeleting means the backup and all its associated data are being deleted.
	BackupPhaseDeleting BackupPhase = "Deleting"
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
	//
	// Deprecated: this field is considered read-only as of v0.10
	// and will be removed in a subsequent release. The information
	// previously contained here is now stored in a file in backup
	// storage.
	VolumeBackups map[string]*VolumeBackupInfo `json:"volumeBackups,omitempty"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable).
	ValidationErrors []string `json:"validationErrors"`

	// StartTimestamp records the time a backup was started.
	// Separate from CreationTimestamp, since that value changes
	// on restores.
	// The server's time is used for StartTimestamps
	StartTimestamp metav1.Time `json:"startTimestamp"`

	// CompletionTimestamp records the time a backup was completed.
	// Completion time is recorded even on failed backups.
	// Completion time is recorded before uploading the backup object.
	// The server's time is used for CompletionTimestamps
	CompletionTimestamp metav1.Time `json:"completionTimestamp"`

	// VolumeSnapshotsAttempted is the total number of attempted
	// volume snapshots for this backup.
	VolumeSnapshotsAttempted int `json:"volumeSnapshotsAttempted"`

	// VolumeSnapshotsCompleted is the total number of successfully
	// completed volume snapshots for this backup.
	VolumeSnapshotsCompleted int `json:"volumeSnapshotsCompleted"`
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

	// AvailabilityZone is the where the volume is provisioned
	// in the cloud provider.
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// Iops is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	Iops *int64 `json:"iops,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backup is an Ark resource that respresents the capture of Kubernetes
// cluster state at a point in time (API objects and associated volume state).
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BackupSpec   `json:"spec"`
	Status BackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList is a list of Backups.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Backup `json:"items"`
}
