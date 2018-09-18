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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VolumeSnapshotLocation is a location where Ark stores volume snapshots.
type VolumeSnapshotLocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   VolumeSnapshotLocationSpec   `json:"spec"`
	Status VolumeSnapshotLocationStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VolumeSnapshotLocationList is a list of VolumeSnapshotLocations.
type VolumeSnapshotLocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []VolumeSnapshotLocation `json:"items"`
}

// VolumeSnapshotLocationSpec defines the specification for an Ark VolumeSnapshotLocation.
type VolumeSnapshotLocationSpec struct {
	// Provider is the provider of the volume storage.
	Provider string `json:"provider"`

	// Config is for provider-specific configuration fields.
	Config map[string]string `json:"config"`
}

// VolumeSnapshotLocationPhase is the lifecyle phase of an Ark VolumeSnapshotLocation.
type VolumeSnapshotLocationPhase string

const (
	// VolumeSnapshotLocationPhaseAvailable means the location is available to read and write from.
	VolumeSnapshotLocationPhaseAvailable VolumeSnapshotLocationPhase = "Available"

	// VolumeSnapshotLocationPhaseUnavailable means the location is unavailable to read and write from.
	VolumeSnapshotLocationPhaseUnavailable VolumeSnapshotLocationPhase = "Unavailable"
)

// VolumeSnapshotLocationStatus describes the current status of an Ark VolumeSnapshotLocation.
type VolumeSnapshotLocationStatus struct {
	Phase VolumeSnapshotLocationPhase `json:"phase,omitempty"`
}
