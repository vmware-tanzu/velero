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

// ScheduleSpec defines the specification for an Ark schedule
type ScheduleSpec struct {
	// Template is the definition of the Backup to be run
	// on the provided schedule
	Template BackupSpec `json:"template"`

	// Schedule is a Cron expression defining when to run
	// the Backup.
	Schedule string `json:"schedule"`
}

// SchedulePhase is a string representation of the lifecycle phase
// of an Ark schedule
type SchedulePhase string

const (
	// SchedulePhaseNew means the schedule has been created but not
	// yet processed by the ScheduleController
	SchedulePhaseNew SchedulePhase = "New"

	// SchedulePhaseEnabled means the schedule has been validated and
	// will now be triggering backups according to the schedule spec.
	SchedulePhaseEnabled SchedulePhase = "Enabled"

	// SchedulePhaseFailedValidation means the schedule has failed
	// the controller's validations and therefore will not trigger backups.
	SchedulePhaseFailedValidation SchedulePhase = "FailedValidation"
)

// ScheduleStatus captures the current state of an Ark schedule
type ScheduleStatus struct {
	// Phase is the current phase of the Schedule
	Phase SchedulePhase `json:"phase"`

	// LastBackup is the last time a Backup was run for this
	// Schedule schedule
	LastBackup metav1.Time `json:"lastBackup"`

	// ValidationErrors is a slice of all validation errors (if
	// applicable)
	ValidationErrors []string `json:"validationErrors"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Schedule is an Ark resource that represents a pre-scheduled or
// periodic Backup that should be run.
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ScheduleSpec   `json:"spec"`
	Status ScheduleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScheduleList is a list of Schedules.
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Schedule `json:"items"`
}
