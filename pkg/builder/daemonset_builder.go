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

// DaemonsetBuilder builds Daemonset objects.
type DaemonsetBuilder struct {
	object *appsv1api.DaemonSet
}

// ForDaemonset is the constructor for a DaemonsetBuilder.
func ForDaemonset(ns, name string) *DaemonsetBuilder {
	return &DaemonsetBuilder{
		object: &appsv1api.DaemonSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1api.SchemeGroupVersion.String(),
				Kind:       "DaemonSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// ForDaemonsetWithImage is the constructor for a DaemonsetBuilder with container image.
func ForDaemonsetWithImage(ns, name string, images ...string) *DaemonsetBuilder {
	containers := []corev1api.Container{}
	for i, image := range images {
		containers = append(containers, corev1api.Container{Name: strconv.Itoa(i), Image: image})
	}
	spec := corev1api.PodSpec{Containers: containers}
	return &DaemonsetBuilder{
		object: &appsv1api.DaemonSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1api.SchemeGroupVersion.String(),
				Kind:       "DaemonSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: appsv1api.DaemonSetSpec{Template: corev1api.PodTemplateSpec{Spec: spec}},
		},
	}
}

// Result returns the built DaemonSet.
func (b *DaemonsetBuilder) Result() *appsv1api.DaemonSet {
	return b.object
}
