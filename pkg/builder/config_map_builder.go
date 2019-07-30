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

// ConfigMapBuilder builds ConfigMap objects.
type ConfigMapBuilder struct {
	object *corev1api.ConfigMap
}

// ForConfigMap is the constructor for a ConfigMapBuilder.
func ForConfigMap(ns, name string) *ConfigMapBuilder {
	return &ConfigMapBuilder{
		object: &corev1api.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built ConfigMap.
func (b *ConfigMapBuilder) Result() *corev1api.ConfigMap {
	return b.object
}

// ObjectMeta applies functional options to the ConfigMap's ObjectMeta.
func (b *ConfigMapBuilder) ObjectMeta(opts ...ObjectMetaOpt) *ConfigMapBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// Data set's the ConfigMap's data.
func (b *ConfigMapBuilder) Data(vals ...string) *ConfigMapBuilder {
	b.object.Data = setMapEntries(b.object.Data, vals...)
	return b
}
