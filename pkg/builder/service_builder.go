/*
Copyright the Velero contributors.

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

// ServiceBuilder builds Service objects.
type ServiceBuilder struct {
	object *corev1api.Service
}

// ForService is the constructor for a ServiceBuilder.
func ForService(ns, name string) *ServiceBuilder {
	return &ServiceBuilder{
		object: &corev1api.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built Service.
func (s *ServiceBuilder) Result() *corev1api.Service {
	return s.object
}

// ObjectMeta applies functional options to the Service's ObjectMeta.
func (s *ServiceBuilder) ObjectMeta(opts ...ObjectMetaOpt) *ServiceBuilder {
	for _, opt := range opts {
		opt(s.object)
	}

	return s
}
