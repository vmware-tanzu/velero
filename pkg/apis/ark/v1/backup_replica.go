package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupReplica is a copy of a backup stored in a particular
// backup storage provider/location.
type BackupReplica struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BackupReplicaSpec   `json:"spec"`
	Status BackupReplicaStatus `json:"status,omitempty"`
}

type BackupReplicaSpec struct {
	// BackupRef is a reference to the backup that this object
	// is a replica of.
	BackupRef corev1.LocalObjectReference `json:"backup"`

	// BackupStorageProviderRef is a reference to the backup
	// storage provider where this replica is stored.
	BackupStorageProviderRef corev1.LocalObjectReference `json:"provider"`

	// Location is the name of the location within the backup
	// storage provider where this replica is stored.
	Location string `json:"location"`
}

type BackupReplicaStatus struct {
	Phase BackupReplicaPhase `json:"phase"`
}

type BackupReplicaPhase string

const (
	BackupReplicaPhaseNew              BackupReplicaPhase = "New"
	BackupReplicaPhaseFailedValidation BackupReplicaPhase = "FailedValidation"
	BackupReplicaPhaseInProgress       BackupReplicaPhase = "InProgress"
	BackupReplicaPhaseCompleted        BackupReplicaPhase = "Completed"
)
