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
)

// SecretKeySelectorBuilder builds SecretKeySelector objects.
type SecretKeySelectorBuilder struct {
	object *corev1api.SecretKeySelector
}

// ForSecretKeySelector is the constructor for a SecretKeySelectorBuilder.
func ForSecretKeySelector(name string, key string) *SecretKeySelectorBuilder {
	return &SecretKeySelectorBuilder{
		object: &corev1api.SecretKeySelector{
			LocalObjectReference: corev1api.LocalObjectReference{
				Name: name,
			},
			Key: key,
		},
	}
}

// Result returns the built SecretKeySelector.
func (b *SecretKeySelectorBuilder) Result() *corev1api.SecretKeySelector {
	return b.object
}
