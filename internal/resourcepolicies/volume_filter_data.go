package resourcepolicies

import (
	corev1api "k8s.io/api/core/v1"
)

// VolumeFilterData bundles the volume data needed for volume policy filtering
type VolumeFilterData struct {
	PersistentVolume *corev1api.PersistentVolume
	PodVolume        *corev1api.Volume
	PVC              *corev1api.PersistentVolumeClaim
}

// NewVolumeFilterData constructs a new VolumeFilterData instance.
func NewVolumeFilterData(pv *corev1api.PersistentVolume, podVol *corev1api.Volume, pvc *corev1api.PersistentVolumeClaim) VolumeFilterData {
	return VolumeFilterData{
		PersistentVolume: pv,
		PodVolume:        podVol,
		PVC:              pvc,
	}
}
