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
	"k8s.io/apimachinery/pkg/api/resource"
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

// Phase sets the PersistentVolumeClaim's status Phase.
func (b *PersistentVolumeClaimBuilder) Phase(phase corev1api.PersistentVolumeClaimPhase) *PersistentVolumeClaimBuilder {
	b.object.Status.Phase = phase
	return b
}

// RequestResource sets the PersistentVolumeClaim's spec.Resources.Requests.
func (b *PersistentVolumeClaimBuilder) RequestResource(requests corev1api.ResourceList) *PersistentVolumeClaimBuilder {
	if b.object.Spec.Resources.Requests == nil {
		b.object.Spec.Resources.Requests = make(map[corev1api.ResourceName]resource.Quantity)
	}
	b.object.Spec.Resources.Requests = requests
	return b
}

// LimitResource sets the PersistentVolumeClaim's spec.Resources.Limits.
func (b *PersistentVolumeClaimBuilder) LimitResource(limits corev1api.ResourceList) *PersistentVolumeClaimBuilder {
	if b.object.Spec.Resources.Limits == nil {
		b.object.Spec.Resources.Limits = make(map[corev1api.ResourceName]resource.Quantity)
	}
	b.object.Spec.Resources.Limits = limits
	return b
}

// DataSource sets the PersistentVolumeClaim's spec.DataSource.
func (b *PersistentVolumeClaimBuilder) DataSource(dataSource *corev1api.TypedLocalObjectReference) *PersistentVolumeClaimBuilder {
	b.object.Spec.DataSource = dataSource
	return b
}

// DataSourceRef sets the PersistentVolumeClaim's spec.DataSourceRef.
func (b *PersistentVolumeClaimBuilder) DataSourceRef(dataSourceRef *corev1api.TypedObjectReference) *PersistentVolumeClaimBuilder {
	b.object.Spec.DataSourceRef = dataSourceRef
	return b
}

// Selector sets the PersistentVolumeClaim's spec.Selector.
func (b *PersistentVolumeClaimBuilder) Selector(labelSelector *metav1.LabelSelector) *PersistentVolumeClaimBuilder {
	b.object.Spec.Selector = labelSelector
	return b
}
