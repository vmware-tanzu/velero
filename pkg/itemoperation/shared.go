/*
Copyright the Velero contributors.

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

package itemoperation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationPhase is the lifecycle phase of a Velero item operation
type OperationPhase string

type OperationStatus struct {
	// Phase is the current state of the item operation.
	Phase OperationPhase `json:"phase,omitempty"`

	// Error displays the reason for a failed operation
	Error string `json:"error,omitempty"`

	// Amount of operation completed (measured in OperationUnits)
	// i.e. number of bytes transferred for a volume
	NCompleted int64 `json:"nCompleted,omitempty"`

	// Total Amount of operation (measured in OperationUnits)
	// i.e. volume size in bytes
	NTotal int64 `json:"nTotal,omitempty"`

	// Units that NCompleted,NTotal are measured in
	// i.e. "bytes"
	OperationUnits string `json:"operationUnits,omitempty"`

	// Description of progress made
	// i.e. "processing", "Current phase: Running", etc.
	Description string `json:"description,omitempty"`

	// Created records the time the item operation was created
	Created *metav1.Time `json:"created,omitempty"`

	// Started records the time the item operation was started, if known
	// +optional
	// +nullable
	Started *metav1.Time `json:"started,omitempty"`

	// Updated records the time the item operation was updated, if known.
	// +optional
	// +nullable
	Updated *metav1.Time `json:"updated,omitempty"`
}

func (in *OperationStatus) DeepCopy() *OperationStatus {
	if in == nil {
		return nil
	}
	out := new(OperationStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *OperationStatus) DeepCopyInto(out *OperationStatus) {
	*out = *in
	if in.Created != nil {
		in, out := &in.Created, &out.Created
		*out = (*in).DeepCopy()
	}
	if in.Started != nil {
		in, out := &in.Started, &out.Started
		*out = (*in).DeepCopy()
	}
	if in.Updated != nil {
		in, out := &in.Updated, &out.Updated
		*out = (*in).DeepCopy()
	}
}

const (
	// OperationPhaseNew means the item operation has been created but not started
	// by the plugin
	OperationPhaseNew OperationPhase = "New"

	// OperationPhaseInProgress means the item operation has been created and started
	// by the plugin
	OperationPhaseInProgress OperationPhase = "InProgress"

	// OperationPhaseCompleted means the item operation was successfully completed
	// and can be used for restore.
	OperationPhaseCompleted OperationPhase = "Completed"

	// OperationPhaseFailed means the item operation ended with an error.
	OperationPhaseFailed OperationPhase = "Failed"
)
