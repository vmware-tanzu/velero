package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   VolumeSnapshotSpec   `json:"spec"`
	Status VolumeSnapshotStatus `json:"status,omitempty"`
}

type VolumeSnapshotSpec struct {
	// VolumeSnapshotProviderRef is a reference to the volume snapshot provider that
	// is used to store this volume snapshot.
	VolumeSnapshotProviderRef corev1.LocalObjectReference `json:"volumeSnapshotProviderRef"`

	// Location is the name of the location within the volume snapshot provider
	// where this snapshot is stored.
	Location string `json:"location"`

	// Type is the type of the disk/volume in the cloud provider
	// API.
	Type string `json:"type"`

	// AvailabilityZone is the where the source volume is provisioned
	// in the cloud provider.
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// Iops is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	Iops *int64 `json:"iops,omitempty"`
}

type VolumeSnapshotStatus struct {
	Phase VolumeSnapshotPhase `json:"phase"`

	// SnapshotID is the ID of the snapshot taken in the cloud
	// provider API of this volume.
	SnapshotID string `json:"snapshotID"`
}

type VolumeSnapshotPhase string

const (
	VolumeSnapshotPhaseNew        = "New"
	VolumeSnapshotPhaseInProgress = "InProgress"
	VolumeSnapshotPhaseCompleted  = "Completed"
)
