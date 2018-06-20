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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResticRepositorySpec is the specification for a ResticRepository.
type ResticRepositorySpec struct {
	// MaintenanceFrequency is how often maintenance should be run.
	MaintenanceFrequency metav1.Duration `json:"maintenanceFrequency"`

	// ResticIdentifier is the full restic-compatible string for identifying
	// this repository.
	ResticIdentifier string `json:"resticIdentifier"`
}

// ResticRepositoryPhase represents the lifecycle phase of a ResticRepository.
type ResticRepositoryPhase string

const (
	ResticRepositoryPhaseNew      ResticRepositoryPhase = "New"
	ResticRepositoryPhaseReady    ResticRepositoryPhase = "Ready"
	ResticRepositoryPhaseNotReady ResticRepositoryPhase = "NotReady"
)

// ResticRepositoryStatus is the current status of a ResticRepository.
type ResticRepositoryStatus struct {
	// Phase is the current state of the ResticRepository.
	Phase ResticRepositoryPhase `json:"phase"`

	// Message is a message about the current status of the ResticRepository.
	Message string `json:"message"`

	// LastMaintenanceTime is the last time maintenance was run.
	LastMaintenanceTime metav1.Time `json:"lastMaintenanceTime"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ResticRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ResticRepositorySpec   `json:"spec"`
	Status ResticRepositoryStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResticRepositoryList is a list of ResticRepositories.
type ResticRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ResticRepository `json:"items"`
}
