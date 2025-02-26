package resourcepolicies

import (
	corev1 "k8s.io/api/core/v1"
)

// VolumeFilterData bundles the volume data needed for volume policy filtering
type VolumeFilterData struct {
	PersistentVolume *corev1.PersistentVolume
	PodVolume        *corev1.Volume
	PVC              *corev1.PersistentVolumeClaim
}

// NewVolumeFilterData constructs a new VolumeFilterData instance.
func NewVolumeFilterData(pv *corev1.PersistentVolume, podVol *corev1.Volume, pvc *corev1.PersistentVolumeClaim) VolumeFilterData {
	return VolumeFilterData{
		PersistentVolume: pv,
		PodVolume:        podVol,
		PVC:              pvc,
	}
}
