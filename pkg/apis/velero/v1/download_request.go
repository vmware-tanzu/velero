/*
Copyright 2017 the Velero contributors.

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
// +kubebuilder:validation:Enum=BackupLog;BackupContents;BackupVolumeSnapshots;BackupResourceList;RestoreLog;RestoreResults
type DownloadTargetKind string

const (
	DownloadTargetKindBackupLog             DownloadTargetKind = "BackupLog"
	DownloadTargetKindBackupContents        DownloadTargetKind = "BackupContents"
	DownloadTargetKindBackupVolumeSnapshots DownloadTargetKind = "BackupVolumeSnapshots"
	DownloadTargetKindBackupResourceList    DownloadTargetKind = "BackupResourceList"
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
// +kubebuilder:validation:Enum=New;Processed
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
	// +optional
	Phase DownloadRequestPhase `json:"phase,omitempty"`

	// DownloadURL contains the pre-signed URL for the target file.
	// +optional
	DownloadURL string `json:"downloadURL,omitempty"`

	// Expiration is when this DownloadRequest expires and can be deleted by the system.
	// +optional
	// +nullable
	Expiration *metav1.Time `json:"expiration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="DownloadRequest status such as New/Processed"
// +kubebuilder:printcolumn:name="Target Name",type="string",JSONPath=".spec.target.name",description="Name of the associated Kubernetes resource"
// +kubebuilder:printcolumn:name="Target Kind",type="string",JSONPath=".spec.target.kind",description="Type of file to download"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DownloadRequest is a request to download an artifact from backup object storage, such as a backup
// log file.
type DownloadRequest struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec DownloadRequestSpec `json:"spec,omitempty"`

	// +optional
	Status DownloadRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DownloadRequestList is a list of DownloadRequests.
type DownloadRequestList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []DownloadRequest `json:"items"`
}
