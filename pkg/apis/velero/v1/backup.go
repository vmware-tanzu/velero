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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the specification for a Velero backup.
type BackupSpec struct {
	// IncludedNamespaces is a slice of namespace names to include objects
	// from. If empty, all namespaces are included.
	// +optional
	// +nullable
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces contains a list of namespaces that are not
	// included in the backup.
	// +optional
	// +nullable
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedResources is a slice of resource names to include
	// in the backup. If empty, all resources are included.
	// +optional
	// +nullable
	IncludedResources []string `json:"includedResources,omitempty"`

	// ExcludedResources is a slice of resource names that are not
	// included in the backup.
	// +optional
	// +nullable
	ExcludedResources []string `json:"excludedResources,omitempty"`

	// LabelSelector is a metav1.LabelSelector to filter with
	// when adding individual objects to the backup. If empty
	// or nil, all objects are included. Optional.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// SnapshotVolumes specifies whether to take cloud snapshots
	// of any PV's referenced in the set of objects included
	// in the Backup.
	// +optional
	// +nullable
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`

	// IncludeClusterResources specifies whether cluster-scoped resources
	// should be included for consideration in the backup.
	// +optional
	// +nullable
	IncludeClusterResources *bool `json:"includeClusterResources,omitempty"`

	// Hooks represent custom behaviors that should be executed at different phases of the backup.
	// +optional
	Hooks BackupHooks `json:"hooks,omitempty"`

	// StorageLocation is a string containing the name of a BackupStorageLocation where the backup should be stored.
	// +optional
	StorageLocation string `json:"storageLocation,omitempty"`

	// VolumeSnapshotLocations is a list containing names of VolumeSnapshotLocations associated with this backup.
	// +optional
	VolumeSnapshotLocations []string `json:"volumeSnapshotLocations,omitempty"`

	// DefaultVolumesToRestic specifies whether restic should be used to take a
	// backup of all pod volumes by default.
	// +optional
	// + nullable
	DefaultVolumesToRestic *bool `json:"defaultVolumesToRestic,omitempty"`

	// OrderedResources specifies the backup order of resources of specific Kind.
	// The map key is the Kind name and value is a list of resource names separated by commas.
	// Each resource name has format "namespace/resourcename".  For cluster resources, simply use "resourcename".
	// +optional
	// +nullable
	OrderedResources map[string]string `json:"orderedResources,omitempty"`
}

// BackupHooks contains custom behaviors that should be executed at different phases of the backup.
type BackupHooks struct {
	// Resources are hooks that should be executed when backing up individual instances of a resource.
	// +optional
	// +nullable
	Resources []BackupResourceHookSpec `json:"resources,omitempty"`
}

// BackupResourceHookSpec defines one or more BackupResourceHooks that should be executed based on
// the rules defined for namespaces, resources, and label selector.
type BackupResourceHookSpec struct {
	// Name is the name of this hook.
	Name string `json:"name"`

	// IncludedNamespaces specifies the namespaces to which this hook spec applies. If empty, it applies
	// to all namespaces.
	// +optional
	// +nullable
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces specifies the namespaces to which this hook spec does not apply.
	// +optional
	// +nullable
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedResources specifies the resources to which this hook spec applies. If empty, it applies
	// to all resources.
	// +optional
	// +nullable
	IncludedResources []string `json:"includedResources,omitempty"`

	// ExcludedResources specifies the resources to which this hook spec does not apply.
	// +optional
	// +nullable
	ExcludedResources []string `json:"excludedResources,omitempty"`

	// LabelSelector, if specified, filters the resources to which this hook spec applies.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// PreHooks is a list of BackupResourceHooks to execute prior to storing the item in the backup.
	// These are executed before any "additional items" from item actions are processed.
	// +optional
	PreHooks []BackupResourceHook `json:"pre,omitempty"`

	// PostHooks is a list of BackupResourceHooks to execute after storing the item in the backup.
	// These are executed after all "additional items" from item actions are processed.
	// +optional
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
	// +optional
	Container string `json:"container,omitempty"`

	// Command is the command and arguments to execute.
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// OnError specifies how Velero should behave if it encounters an error executing this hook.
	// +optional
	OnError HookErrorMode `json:"onError,omitempty"`

	// Timeout defines the maximum amount of time Velero should wait for the hook to complete before
	// considering the execution a failure.
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// HookErrorMode defines how Velero should treat an error from a hook.
// +kubebuilder:validation:Enum=Continue;Fail
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
// of a Velero backup.
// +kubebuilder:validation:Enum=New;FailedValidation;InProgress;Completed;PartiallyFailed;Failed;Deleting
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

	// BackupPhasePartiallyFailed means the backup has run to completion
	// but encountered 1+ errors backing up individual items.
	BackupPhasePartiallyFailed BackupPhase = "PartiallyFailed"

	// BackupPhaseFailed means the backup ran but encountered an error that
	// prevented it from completing successfully.
	BackupPhaseFailed BackupPhase = "Failed"

	// BackupPhaseDeleting means the backup and all its associated data are being deleted.
	BackupPhaseDeleting BackupPhase = "Deleting"
)

// BackupStatus captures the current status of a Velero backup.
type BackupStatus struct {
	// Version is the backup format major version.
	// Deprecated: Please see FormatVersion
	// +optional
	Version int `json:"version,omitempty"`

	// FormatVersion is the backup format version, including major, minor, and patch version.
	// +optional
	FormatVersion string `json:"formatVersion,omitempty"`

	// Expiration is when this Backup is eligible for garbage-collection.
	// +optional
	// +nullable
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// Phase is the current state of the Backup.
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable).
	// +optional
	// +nullable
	ValidationErrors []string `json:"validationErrors,omitempty"`

	// StartTimestamp records the time a backup was started.
	// Separate from CreationTimestamp, since that value changes
	// on restores.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a backup was completed.
	// Completion time is recorded even on failed backups.
	// Completion time is recorded before uploading the backup object.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// VolumeSnapshotsAttempted is the total number of attempted
	// volume snapshots for this backup.
	// +optional
	VolumeSnapshotsAttempted int `json:"volumeSnapshotsAttempted,omitempty"`

	// VolumeSnapshotsCompleted is the total number of successfully
	// completed volume snapshots for this backup.
	// +optional
	VolumeSnapshotsCompleted int `json:"volumeSnapshotsCompleted,omitempty"`

	// Warnings is a count of all warning messages that were generated during
	// execution of the backup. The actual warnings are in the backup's log
	// file in object storage.
	// +optional
	Warnings int `json:"warnings,omitempty"`

	// Errors is a count of all error messages that were generated during
	// execution of the backup.  The actual errors are in the backup's log
	// file in object storage.
	// +optional
	Errors int `json:"errors,omitempty"`

	// Progress contains information about the backup's execution progress. Note
	// that this information is best-effort only -- if Velero fails to update it
	// during a backup for any reason, it may be inaccurate/stale.
	// +optional
	// +nullable
	Progress *BackupProgress `json:"progress,omitempty"`
}

// BackupProgress stores information about the progress of a Backup's execution.
type BackupProgress struct {
	// TotalItems is the total number of items to be backed up. This number may change
	// throughout the execution of the backup due to plugins that return additional related
	// items to back up, the velero.io/exclude-from-backup label, and various other
	// filters that happen as items are processed.
	// +optional
	TotalItems int `json:"totalItems,omitempty"`

	// ItemsBackedUp is the number of items that have actually been written to the
	// backup tarball so far.
	// +optional
	ItemsBackedUp int `json:"itemsBackedUp,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Backup status such as InProgress/Completed"
// +kubebuilder:printcolumn:name="Errors",type="integer",JSONPath=".status.errors",description="Count of all error messages that were generated during execution of this backup"
// +kubebuilder:printcolumn:name="Warnings",type="integer",JSONPath=".status.warnings",description="Count of all warning messages that were generated during execution of this backup"
// +kubebuilder:printcolumn:name="Created",type="date",JSONPath=".status.startTimestamp",description="Time when this backup was started"
// +kubebuilder:printcolumn:name="Storage Location",type="string",JSONPath=".spec.storageLocation",description="Name of the Backup Storage Location where this backup should be stored"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backup is a Velero resource that represents the capture of Kubernetes
// cluster state at a point in time (API objects and associated volume state).
type Backup struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec BackupSpec `json:"spec,omitempty"`

	// +optional
	Status BackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList is a list of Backups.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Backup `json:"items"`
}
