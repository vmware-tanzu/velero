/*
Copyright 2022 the Velero contributors.

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
	"strconv"

	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicaSetBuilder builds ReplicaSet objects.
type ReplicaSetBuilder struct {
	object *appsv1api.ReplicaSet
}

// ForReplicaSet is the constructor for a ReplicaSetBuilder.
func ForReplicaSet(ns, name string) *ReplicaSetBuilder {
	return &ReplicaSetBuilder{
		object: &appsv1api.ReplicaSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1api.SchemeGroupVersion.String(),
				Kind:       "ReplicaSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// ForReplicaSetWithImage is the constructor for a ReplicaSetBuilder with container image.
func ForReplicaSetWithImage(ns, name string, images ...string) *ReplicaSetBuilder {
	containers := []corev1api.Container{}
	for i, image := range images {
		containers = append(containers, corev1api.Container{Name: strconv.Itoa(i), Image: image})
	}
	spec := corev1api.PodSpec{Containers: containers}
	return &ReplicaSetBuilder{
		object: &appsv1api.ReplicaSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1api.SchemeGroupVersion.String(),
				Kind:       "ReplicaSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: appsv1api.ReplicaSetSpec{Template: corev1api.PodTemplateSpec{Spec: spec}},
		},
	}
}

// Result returns the built ReplicaSet.
func (b *ReplicaSetBuilder) Result() *appsv1api.ReplicaSet {
	return b.object
}

// ObjectMeta applies functional options to the ReplicaSet's ObjectMeta.
func (b *ReplicaSetBuilder) ObjectMeta(opts ...ObjectMetaOpt) *ReplicaSetBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
