/*
Copyright 2021 the Velero contributors.

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetBuilder builds StatefulSet objects.
type StatefulSetBuilder struct {
	object *appsv1.StatefulSet
}

// ForStatefulSet is the constructor for a StatefulSetBuilder.
func ForStatefulSet(ns, name string) *StatefulSetBuilder {
	return &StatefulSetBuilder{
		object: &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "StatefulSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: appsv1.StatefulSetSpec{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{},
			},
		},
	}
}

// Result returns the built StatefulSet.
func (b *StatefulSetBuilder) Result() *appsv1.StatefulSet {
	return b.object
}

// StorageClass sets the StatefulSet's VolumeClaimTemplates storage class name.
func (b *StatefulSetBuilder) StorageClass(names ...string) *StatefulSetBuilder {
	for _, name := range names {
		nameTmp := name
		b.object.Spec.VolumeClaimTemplates = append(b.object.Spec.VolumeClaimTemplates,
			corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &nameTmp}})
	}
	return b
}
