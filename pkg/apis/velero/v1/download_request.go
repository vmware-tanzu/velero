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

// DownloadRequestSpec is the specification for a download request.
type DownloadRequestSpec struct {
	// Target is what to download (e.g. logs for a backup).
	Target DownloadTarget `json:"target"`
}

// DownloadTargetKind represents what type of file to download.
type DownloadTargetKind string

const (
	DownloadTargetKindBackupLog             DownloadTargetKind = "BackupLog"
	DownloadTargetKindBackupContents        DownloadTargetKind = "BackupContents"
	DownloadTargetKindBackupVolumeSnapshots DownloadTargetKind = "BackupVolumeSnapshots"
	DownloadTargetKindRestoreLog            DownloadTargetKind = "RestoreLog"
	DownloadTargetKindRestoreResults        DownloadTargetKind = "RestoreResults"
)

// DownloadTarget is the specification for what kind of file to download, and the name of the
// resource with which it's associated.
type DownloadTarget struct {
	// Kind is the type of file to download.
	Kind DownloadTargetKind `json:"kind"`
	// Name is the name of the kubernetes resource with which the file is associated.
	Name string `json:"name"`
}

// DownloadRequestPhase represents the lifecycle phase of a DownloadRequest.
type DownloadRequestPhase string

const (
	// DownloadRequestPhaseNew means the DownloadRequest has not been processed by the
	// DownloadRequestController yet.
	DownloadRequestPhaseNew DownloadRequestPhase = "New"
	// DownloadRequestPhaseProcessed means the DownloadRequest has been processed by the
	// DownloadRequestController.
	DownloadRequestPhaseProcessed DownloadRequestPhase = "Processed"
)

// DownloadRequestStatus is the current status of a DownloadRequest.
type DownloadRequestStatus struct {
	// Phase is the current state of the DownloadRequest.
	Phase DownloadRequestPhase `json:"phase"`
	// DownloadURL contains the pre-signed URL for the target file.
	DownloadURL string `json:"downloadURL"`
	// Expiration is when this DownloadRequest expires and can be deleted by the system.
	Expiration metav1.Time `json:"expiration"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DownloadRequest is a request to download an artifact from backup object storage, such as a backup
// log file.
type DownloadRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DownloadRequestSpec   `json:"spec"`
	Status DownloadRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DownloadRequestList is a list of DownloadRequests.
type DownloadRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DownloadRequest `json:"items"`
}
