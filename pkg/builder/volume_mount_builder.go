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
)

// VolumeMountBuilder builds VolumeMount objects.
type VolumeMountBuilder struct {
	object *corev1api.VolumeMount
}

// ForVolumeMount is the constructor for a VolumeMountBuilder.
func ForVolumeMount(name, mountPath string) *VolumeMountBuilder {
	return &VolumeMountBuilder{
		object: &corev1api.VolumeMount{
			Name:      name,
			MountPath: mountPath,
		},
	}
}

// Result returns the built VolumeMount.
func (b *VolumeMountBuilder) Result() *corev1api.VolumeMount {
	return b.object
}
