/*
Copyright 2017, 2019 the Velero contributors.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestoreSpec defines the specification for a Velero restore.
type RestoreSpec struct {
	// BackupName is the unique name of the Velero backup to restore
	// from.
	BackupName string `json:"backupName"`

	// ScheduleName is the unique name of the Velero schedule to restore
	// from. If specified, and BackupName is empty, Velero will restore
	// from the most recent successful backup created from this schedule.
	// +optional
	ScheduleName string `json:"scheduleName,omitempty"`

	// IncludedNamespaces is a slice of namespace names to include objects
	// from. If empty, all namespaces are included.
	// +optional
	// +nullable
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces contains a list of namespaces that are not
	// included in the restore.
	// +optional
	// +nullable
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedResources is a slice of resource names to include
	// in the restore. If empty, all resources in the backup are included.
	// +optional
	// +nullable
	IncludedResources []string `json:"includedResources,omitempty"`

	// ExcludedResources is a slice of resource names that are not
	// included in the restore.
	// +optional
	// +nullable
	ExcludedResources []string `json:"excludedResources,omitempty"`

	// NamespaceMapping is a map of source namespace names
	// to target namespace names to restore into. Any source
	// namespaces not included in the map will be restored into
	// namespaces of the same name.
	// +optional
	NamespaceMapping map[string]string `json:"namespaceMapping,omitempty"`

	// LabelSelector is a metav1.LabelSelector to filter with
	// when restoring individual objects from the backup. If empty
	// or nil, all objects are included. Optional.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// RestorePVs specifies whether to restore all included
	// PVs from snapshot (via the cloudprovider).
	// +optional
	// +nullable
	RestorePVs *bool `json:"restorePVs,omitempty"`

	// PreserveNodePorts specifies whether to restore old nodePorts from backup.
	// +optional
	// +nullable
	PreserveNodePorts *bool `json:"preserveNodePorts,omitempty"`

	// IncludeClusterResources specifies whether cluster-scoped resources
	// should be included for consideration in the restore. If null, defaults
	// to true.
	// +optional
	// +nullable
	IncludeClusterResources *bool `json:"includeClusterResources,omitempty"`

	// Hooks represent custom behaviors that should be executed during or post restore.
	// +optional
	Hooks RestoreHooks `json:"hooks,omitempty"`
}

// RestoreHooks contains custom behaviors that should be executed during or post restore.
type RestoreHooks struct {
	Resources []RestoreResourceHookSpec `json:"resources,omitempty"`
}

// RestoreResourceHookSpec defines one or more RestoreResrouceHooks that should be executed based on
// the rules defined for namespaces, resources, and label selector.
type RestoreResourceHookSpec struct {
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

	// PostHooks is a list of RestoreResourceHooks to execute during and after restoring a resource.
	// +optional
	PostHooks []RestoreResourceHook `json:"postHooks,omitempty"`
}

// RestoreResourceHook defines a restore hook for a resource.
type RestoreResourceHook struct {
	// Exec defines an exec restore hook.
	Exec *ExecRestoreHook `json:"exec,omitempty"`

	// Init defines an init restore hook.
	Init *InitRestoreHook `json:"init,omitempty"`
}

// ExecRestoreHook is a hook that uses pod exec API to execute a command inside a container in a pod
type ExecRestoreHook struct {
	// Container is the container in the pod where the command should be executed. If not specified,
	// the pod's first container is used.
	// +optional
	Container string `json:"container,omitempty"`

	// Command is the command and arguments to execute from within a container after a pod has been restored.
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// OnError specifies how Velero should behave if it encounters an error executing this hook.
	// +optional
	OnError HookErrorMode `json:"onError,omitempty"`

	// ExecTimeout defines the maximum amount of time Velero should wait for the hook to complete before
	// considering the execution a failure.
	// +optional
	ExecTimeout metav1.Duration `json:"execTimeout,omitempty"`

	// WaitTimeout defines the maximum amount of time Velero should wait for the container to be Ready
	// before attempting to run the command.
	// +optional
	WaitTimeout metav1.Duration `json:"waitTimeout,omitempty"`
}

// InitRestoreHook is a hook that adds an init container to a PodSpec to run commands before the
// workload pod is able to start.
type InitRestoreHook struct {
	// InitContainers is list of init containers to be added to a pod during its restore.
	// +optional
	InitContainers []v1.Container `json:"initContainers"`

	// Timeout defines the maximum amount of time Velero should wait for the initContainers to complete.
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// RestorePhase is a string representation of the lifecycle phase
// of a Velero restore
// +kubebuilder:validation:Enum=New;FailedValidation;InProgress;Completed;PartiallyFailed;Failed
type RestorePhase string

const (
	// RestorePhaseNew means the restore has been created but not
	// yet processed by the RestoreController
	RestorePhaseNew RestorePhase = "New"

	// RestorePhaseFailedValidation means the restore has failed
	// the controller's validations and therefore will not run.
	RestorePhaseFailedValidation RestorePhase = "FailedValidation"

	// RestorePhaseInProgress means the restore is currently executing.
	RestorePhaseInProgress RestorePhase = "InProgress"

	// RestorePhaseCompleted means the restore has run successfully
	// without errors.
	RestorePhaseCompleted RestorePhase = "Completed"

	// RestorePhasePartiallyFailed means the restore has run to completion
	// but encountered 1+ errors restoring individual items.
	RestorePhasePartiallyFailed RestorePhase = "PartiallyFailed"

	// RestorePhaseFailed means the restore was unable to execute.
	// The failing error is recorded in status.FailureReason.
	RestorePhaseFailed RestorePhase = "Failed"
)

// RestoreStatus captures the current status of a Velero restore
type RestoreStatus struct {
	// Phase is the current state of the Restore
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable)
	// +optional
	// +nullable
	ValidationErrors []string `json:"validationErrors,omitempty"`

	// Warnings is a count of all warning messages that were generated during
	// execution of the restore. The actual warnings are stored in object storage.
	// +optional
	Warnings int `json:"warnings,omitempty"`

	// Errors is a count of all error messages that were generated during
	// execution of the restore. The actual errors are stored in object storage.
	// +optional
	Errors int `json:"errors,omitempty"`

	// FailureReason is an error that caused the entire restore to fail.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// StartTimestamp records the time the restore operation was started.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time the restore operation was completed.
	// Completion time is recorded even on failed restore.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".spec.backupName",description="Name of backup to restore from"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Restore status such as InProgress/Completed"
// +kubebuilder:printcolumn:name="Started",type="date",JSONPath=".status.startTimestamp",description="Time when the restore operation was started"
// +kubebuilder:printcolumn:name="Completed",type="date",JSONPath=".status.completionTimestamp",description="Time when the restore operation was completed"
// +kubebuilder:printcolumn:name="Errors",type="integer",JSONPath=".status.errors",description="Count of all errors generated during the restore"
// +kubebuilder:printcolumn:name="Warnings",type="integer",JSONPath=".status.warnings",description="Count of all warnings generated during the restore"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Restore is a Velero resource that represents the application of
// resources from a Velero backup to a target Kubernetes cluster.
type Restore struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec RestoreSpec `json:"spec,omitempty"`

	// +optional
	Status RestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RestoreList is a list of Restores.
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata"`

	Items []Restore `json:"items"`
}
