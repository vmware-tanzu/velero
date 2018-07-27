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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RestoreSpec defines the specification for an Ark restore.
type RestoreSpec struct {
	// BackupName is the unique name of the Ark backup to restore
	// from.
	BackupName string `json:"backupName"`

	// ScheduleName is the unique name of the Ark schedule to restore
	// from. If specified, and BackupName is empty, Ark will restore
	// from the most recent successful backup created from this schedule.
	ScheduleName string `json:"scheduleName,omitempty"`

	// IncludedNamespaces is a slice of namespace names to include objects
	// from. If empty, all namespaces are included.
	IncludedNamespaces []string `json:"includedNamespaces"`

	// ExcludedNamespaces contains a list of namespaces that are not
	// included in the restore.
	ExcludedNamespaces []string `json:"excludedNamespaces"`

	// IncludedResources is a slice of resource names to include
	// in the restore. If empty, all resources in the backup are included.
	IncludedResources []string `json:"includedResources"`

	// ExcludedResources is a slice of resource names that are not
	// included in the restore.
	ExcludedResources []string `json:"excludedResources"`

	// NamespaceMapping is a map of source namespace names
	// to target namespace names to restore into. Any source
	// namespaces not included in the map will be restored into
	// namespaces of the same name.
	NamespaceMapping map[string]string `json:"namespaceMapping"`

	// LabelSelector is a metav1.LabelSelector to filter with
	// when restoring individual objects from the backup. If empty
	// or nil, all objects are included. Optional.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// RestorePVs specifies whether to restore all included
	// PVs from snapshot (via the cloudprovider).
	RestorePVs *bool `json:"restorePVs,omitempty"`

	// IncludeClusterResources specifies whether cluster-scoped resources
	// should be included for consideration in the restore. If null, defaults
	// to true.
	IncludeClusterResources *bool `json:"includeClusterResources,omitempty"`
}

// RestorePhase is a string representation of the lifecycle phase
// of an Ark restore
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

	// RestorePhaseCompleted means the restore has finished executing.
	// Any relevant warnings or errors will be captured in the Status.
	RestorePhaseCompleted RestorePhase = "Completed"

	// RestorePhaseFailed means the restore was unable to execute.
	// The failing error is recorded in status.FailureReason.
	RestorePhaseFailed RestorePhase = "Failed"
)

// RestoreStatus captures the current status of an Ark restore
type RestoreStatus struct {
	// Phase is the current state of the Restore
	Phase RestorePhase `json:"phase"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable)
	ValidationErrors []string `json:"validationErrors"`

	// Warnings is a count of all warning messages that were generated during
	// execution of the restore. The actual warnings are stored in object storage.
	Warnings int `json:"warnings"`

	// Errors is a count of all error messages that were generated during
	// execution of the restore. The actual errors are stored in object storage.
	Errors int `json:"errors"`

	// FailureReason is an error that caused the entire restore to fail.
	FailureReason string `json:"failureReason"`
}

// RestoreResult is a collection of messages that were generated
// during execution of a restore. This will typically store either
// warning or error messages.
type RestoreResult struct {
	// Ark is a slice of messages related to the operation of Ark
	// itself (for example, messages related to connecting to the
	// cloud, reading a backup file, etc.)
	Ark []string `json:"ark"`

	// Cluster is a slice of messages related to restoring cluster-
	// scoped resources.
	Cluster []string `json:"cluster"`

	// Namespaces is a map of namespace name to slice of messages
	// related to restoring namespace-scoped resources.
	Namespaces map[string][]string `json:"namespaces"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Restore is an Ark resource that represents the application of
// resources from an Ark backup to a target Kubernetes cluster.
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   RestoreSpec   `json:"spec"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RestoreList is a list of Restores.
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Restore `json:"items"`
}
