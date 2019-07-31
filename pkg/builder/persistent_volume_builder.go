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

// PersistentVolumeBuilder builds PersistentVolume objects.
type PersistentVolumeBuilder struct {
	object *corev1api.PersistentVolume
}

// ForPersistentVolume is the constructor for a PersistentVolumeBuilder.
func ForPersistentVolume(name string) *PersistentVolumeBuilder {
	return &PersistentVolumeBuilder{
		object: &corev1api.PersistentVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "PersistentVolume",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Result returns the built PersistentVolume.
func (b *PersistentVolumeBuilder) Result() *corev1api.PersistentVolume {
	return b.object
}

// ObjectMeta applies functional options to the PersistentVolume's ObjectMeta.
func (b *PersistentVolumeBuilder) ObjectMeta(opts ...ObjectMetaOpt) *PersistentVolumeBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// ReclaimPolicy sets the PersistentVolume's reclaim policy.
func (b *PersistentVolumeBuilder) ReclaimPolicy(policy corev1api.PersistentVolumeReclaimPolicy) *PersistentVolumeBuilder {
	b.object.Spec.PersistentVolumeReclaimPolicy = policy
	return b
}

// ClaimRef sets the PersistentVolume's claim ref.
func (b *PersistentVolumeBuilder) ClaimRef(ns, name string) *PersistentVolumeBuilder {
	b.object.Spec.ClaimRef = &corev1api.ObjectReference{
		Namespace: ns,
		Name:      name,
	}
	return b
}

// AWSEBSVolumeID sets the PersistentVolume's AWSElasticBlockStore volume ID.
func (b *PersistentVolumeBuilder) AWSEBSVolumeID(volumeID string) *PersistentVolumeBuilder {
	if b.object.Spec.AWSElasticBlockStore == nil {
		b.object.Spec.AWSElasticBlockStore = new(corev1api.AWSElasticBlockStoreVolumeSource)
	}
	b.object.Spec.AWSElasticBlockStore.VolumeID = volumeID
	return b
}

// CSI sets the PersistentVolume's CSI.
func (b *PersistentVolumeBuilder) CSI(driver, volumeHandle string) *PersistentVolumeBuilder {
	if b.object.Spec.CSI == nil {
		b.object.Spec.CSI = new(corev1api.CSIPersistentVolumeSource)
	}
	b.object.Spec.CSI.Driver = driver
	b.object.Spec.CSI.VolumeHandle = volumeHandle
	return b
}

// StorageClass sets the PersistentVolume's storage class name.
func (b *PersistentVolumeBuilder) StorageClass(name string) *PersistentVolumeBuilder {
	b.object.Spec.StorageClassName = name
	return b
}
