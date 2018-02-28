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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DeleteBackupRequestSpec is the specification for which backups to delete.
type DeleteBackupRequestSpec struct {
	BackupName string `json:"backupName"`
}

// DeleteBackupRequestPhase represents the lifecycle phase of a DeleteBackupRequest.
type DeleteBackupRequestPhase string

const (
	// DeleteBackupRequestPhaseNew means the DeleteBackupRequest has not been processed yet.
	DeleteBackupRequestPhaseNew DeleteBackupRequestPhase = "New"
	// DeleteBackupRequestPhaseInProgress means the DeleteBackupRequest is being processed.
	DeleteBackupRequestPhaseInProgress DeleteBackupRequestPhase = "InProgress"
	// DeleteBackupRequestPhaseProcessed means the DeleteBackupRequest has been processed.
	DeleteBackupRequestPhaseProcessed DeleteBackupRequestPhase = "Processed"
)

// DeleteBackupRequestStatus is the current status of a DeleteBackupRequest.
type DeleteBackupRequestStatus struct {
	// Phase is the current state of the DeleteBackupRequest.
	Phase DeleteBackupRequestPhase `json:"phase"`
	// Errors contains any errors that were encountered during the deletion process.
	Errors []string `json:"errors"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeleteBackupRequest is a request to delete one or more backups.
type DeleteBackupRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DeleteBackupRequestSpec   `json:"spec"`
	Status DeleteBackupRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeleteBackupRequestList is a list of DeleteBackupRequests.
type DeleteBackupRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DeleteBackupRequest `json:"items"`
}
