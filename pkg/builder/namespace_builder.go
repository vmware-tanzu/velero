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

// NamespaceBuilder builds Namespace objects.
type NamespaceBuilder struct {
	object *corev1api.Namespace
}

// ForNamespace is the constructor for a NamespaceBuilder.
func ForNamespace(name string) *NamespaceBuilder {
	return &NamespaceBuilder{
		object: &corev1api.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Result returns the built Namespace.
func (b *NamespaceBuilder) Result() *corev1api.Namespace {
	return b.object
}

// ObjectMeta applies functional options to the Namespace's ObjectMeta.
func (b *NamespaceBuilder) ObjectMeta(opts ...ObjectMetaOpt) *NamespaceBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// Phase sets the namespace's phase
func (b *NamespaceBuilder) Phase(val corev1api.NamespacePhase) *NamespaceBuilder {
	b.object.Status.Phase = val
	return b
}
