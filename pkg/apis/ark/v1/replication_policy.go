package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReplicationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ReplicationPolicySpec   `json:"spec"`
	Status ReplicationPolicyStatus `json:"status,omitempty"`
}

type ReplicationPolicySpec struct {
	// BackupSelector is a label selector that is used to identify backups
	// that should be replicated to the specified locations.
	BackupSelector metav1.LabelSelector `json:"backupSelector"`

	// BackupStorageLocations is a map of backup storage provider reference to
	// a list of locations within the provider that backups should be replicated
	// to.
	BackupStorageLocations map[corev1.LocalObjectReference][]string `json:"backupStorageLocations,omitempty"`

	// VolumeStorageLocations is a map of volume snapshot provider reference to
	// a list of locations within the provider that volume snapshots should be
	// replicated to.
	VolumeStorageLocations map[corev1.LocalObjectReference][]string `json:"volumeStorageLocations,omitempty"`
}

type ReplicationPolicyStatus struct {
	Phase ReplicationPolicyPhase `json:"phase"`
}

type ReplicationPolicyPhase string

const (
	ReplicationPolicyPhaseNew              = "New"
	ReplicationPolicyPhaseFailedValidation = "FailedValidation"
	ReplicationPolicyPhaseReady            = "Ready"
)

func NewReplicationPolicy() *ReplicationPolicy {
	return &ReplicationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: DefaultNamespace,
			Name:      "default",
		},
		Spec: ReplicationPolicySpec{
			BackupSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"replicate": "true",
				},
			},
			BackupStorageLocations: map[corev1.LocalObjectReference][]string{
				corev1.LocalObjectReference{Name: "aws"}: []string{"us-east-1", "us-west-1"},
			},
			VolumeStorageLocations: map[corev1.LocalObjectReference][]string{
				corev1.LocalObjectReference{Name: "aws"}:      []string{"us-east-1", "us-west-1"},
				corev1.LocalObjectReference{Name: "portworx"}: []string{"foo"},
			},
		},
	}
}
