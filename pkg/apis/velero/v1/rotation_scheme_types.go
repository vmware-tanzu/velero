/*
Copyright 2023 the Velero contributors.

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

// RotationSchemeSpec defines the specification for a Velero rotation scheme
// which can be applied to one or more schedules
type RotationSchemeSpec struct {
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=1
	// Number of daily backups to keep. Leave blank to keep all daily backups, or set to 1 or more to keep only the most recent N daily backups.
	KeepDaily *int `json:"keepDaily,omitempty"`
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=1
	// Number of weekly backups to keep. Leave blank to keep all weekly backups, or set to 1 or more to keep only the most recent N weekly backups.
	KeepWeekly *int `json:"keepWeekly,omitempty"`
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=1
	// Number of monthly backups to keep. Leave blank to keep all monthly backups, or set to 1 or more to keep only the most recent N monthly backups.
	KeepMonthly *int `json:"keepMonthly,omitempty"`
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=1
	// Number of yearly backups to keep. Leave blank to keep all yearly backups, or set to 1 or more to keep only the most recent N yearly backups.
	KeepYearly *int `json:"keepYearly,omitempty"`
}

// borrow phase from schedule
type RotationSchemeStatus struct {
	// Last time the rotation scheme was validated
	LastValidatedTimestamp *metav1.Time `json:"lastValidatedTimestamp,omitempty"`
	// Phase of the rotation scheme
	// +kubebuilder:validation:Enum=New;Enabled;FailedValidation
	RotationSchemePhase SchedulePhase `json:"rotationSchemePhase,omitempty"`
	// List of schedules that this rotation scheme is applied to
	AppliedToSchedules []string `json:"appliedToSchedules,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:generate=true
// +kubebuilder:resource:shortName=rs
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Keep Daily",type="integer",JSONPath=".spec.keepDaily"
// +kubebuilder:printcolumn:name="Keep Weekly",type="integer",JSONPath=".spec.keepWeekly"
// +kubebuilder:printcolumn:name="Keep Monthly",type="integer",JSONPath=".spec.keepMonthly"
// +kubebuilder:printcolumn:name="Keep Yearly",type="integer",JSONPath=".spec.keepYearly"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.rotationSchemePhase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Last Validated",type="date",JSONPath=".status.lastValidatedTimestamp"
type RotationScheme struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RotationSchemeSpec   `json:"spec,omitempty"`
	Status            RotationSchemeStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:generate=true
// RotationSchemeList is a collection of RotationSchemes
type RotationSchemeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RotationScheme `json:"items"`
}
