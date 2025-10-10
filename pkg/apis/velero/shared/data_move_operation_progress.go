/*
Copyright The Velero Contributors.

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

package shared

// DataMoveOperationProgress represents the progress of a
// data movement operation

// +k8s:deepcopy-gen=true
type DataMoveOperationProgress struct {
	// +optional
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// +optional
	IncrementalBytes int64 `json:"incrementalBytes,omitempty"`

	// +optional
	BytesDone int64 `json:"bytesDone,omitempty"`
}
