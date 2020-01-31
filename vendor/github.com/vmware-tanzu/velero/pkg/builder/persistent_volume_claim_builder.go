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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersistentVolumeClaimBuilder builds PersistentVolumeClaim objects.
type PersistentVolumeClaimBuilder struct {
	object *corev1api.PersistentVolumeClaim
}

// ForPersistentVolumeClaim is the constructor for a PersistentVolumeClaimBuilder.
func ForPersistentVolumeClaim(ns, name string) *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{
		object: &corev1api.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built PersistentVolumeClaim.
func (b *PersistentVolumeClaimBuilder) Result() *corev1api.PersistentVolumeClaim {
	return b.object
}

// ObjectMeta applies functional options to the PersistentVolumeClaim's ObjectMeta.
func (b *PersistentVolumeClaimBuilder) ObjectMeta(opts ...ObjectMetaOpt) *PersistentVolumeClaimBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// VolumeName sets the PersistentVolumeClaim's volume name.
func (b *PersistentVolumeClaimBuilder) VolumeName(name string) *PersistentVolumeClaimBuilder {
	b.object.Spec.VolumeName = name
	return b
}

// StorageClass sets the PersistentVolumeClaim's storage class name.
func (b *PersistentVolumeClaimBuilder) StorageClass(name string) *PersistentVolumeClaimBuilder {
	b.object.Spec.StorageClassName = &name
	return b
}
