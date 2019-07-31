/*
Copyright 2019 the Velero contributors.

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

package builder

import (
	corev1api "k8s.io/api/core/v1"
)

// VolumeBuilder builds Volume objects.
type VolumeBuilder struct {
	object *corev1api.Volume
}

// ForVolume is the constructor for a VolumeBuilder.
func ForVolume(name string) *VolumeBuilder {
	return &VolumeBuilder{
		object: &corev1api.Volume{
			Name: name,
		},
	}
}

// Result returns the built Volume.
func (b *VolumeBuilder) Result() *corev1api.Volume {
	return b.object
}

// PersistentVolumeClaimSource sets the Volume's persistent volume claim source.
func (b *VolumeBuilder) PersistentVolumeClaimSource(claimName string) *VolumeBuilder {
	b.object.PersistentVolumeClaim = &corev1api.PersistentVolumeClaimVolumeSource{
		ClaimName: claimName,
	}
	return b
}

// CSISource sets the Volume's CSI source.
func (b *VolumeBuilder) CSISource(driver string) *VolumeBuilder {
	b.object.CSI = &corev1api.CSIVolumeSource{
		Driver: driver,
	}
	return b
}
