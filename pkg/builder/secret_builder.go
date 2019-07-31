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

// SecretBuilder builds Secret objects.
type SecretBuilder struct {
	object *corev1api.Secret
}

// ForSecret is the constructor for a SecretBuilder.
func ForSecret(ns, name string) *SecretBuilder {
	return &SecretBuilder{
		object: &corev1api.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1api.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built Secret.
func (b *SecretBuilder) Result() *corev1api.Secret {
	return b.object
}

// ObjectMeta applies functional options to the Secret's ObjectMeta.
func (b *SecretBuilder) ObjectMeta(opts ...ObjectMetaOpt) *SecretBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
