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
	"strings"

	corev1api "k8s.io/api/core/v1"
)

// ContainerBuilder builds Container objects
type ContainerBuilder struct {
	object *corev1api.Container
}

// ForContainer is the constructor for ContainerBuilder.
func ForContainer(name, image string) *ContainerBuilder {
	return &ContainerBuilder{
		object: &corev1api.Container{
			Name:  name,
			Image: image,
		},
	}
}

// ForPluginContainer is a helper builder specifically for plugin init containers
func ForPluginContainer(image string, pullPolicy corev1api.PullPolicy) *ContainerBuilder {
	volumeMount := ForVolumeMount("plugins", "/target").Result()
	return ForContainer(getName(image), image).PullPolicy(pullPolicy).VolumeMounts(volumeMount)
}

// getName returns the 'name' component of a docker
// image (i.e. everything after the last '/' and before
// any subsequent ':')
func getName(image string) string {
	slashIndex := strings.LastIndex(image, "/")
	colonIndex := strings.LastIndex(image, ":")

	start := 0
	if slashIndex > 0 {
		start = slashIndex + 1
	}

	end := len(image)
	if colonIndex > slashIndex {
		end = colonIndex
	}

	return image[start:end]
}

// Result returns the built Container.
func (b *ContainerBuilder) Result() *corev1api.Container {
	return b.object
}

// Args sets the container's Args.
func (b *ContainerBuilder) Args(args ...string) *ContainerBuilder {
	b.object.Args = append(b.object.Args, args...)
	return b
}

// VolumeMounts sets the container's VolumeMounts.
func (b *ContainerBuilder) VolumeMounts(volumeMounts ...*corev1api.VolumeMount) *ContainerBuilder {
	for _, v := range volumeMounts {
		b.object.VolumeMounts = append(b.object.VolumeMounts, *v)
	}
	return b
}

// Resources sets the container's Resources.
func (b *ContainerBuilder) Resources(resources *corev1api.ResourceRequirements) *ContainerBuilder {
	b.object.Resources = *resources
	return b
}

func (b *ContainerBuilder) Env(vars ...*corev1api.EnvVar) *ContainerBuilder {
	for _, v := range vars {
		b.object.Env = append(b.object.Env, *v)
	}
	return b
}

func (b *ContainerBuilder) PullPolicy(pullPolicy corev1api.PullPolicy) *ContainerBuilder {
	b.object.ImagePullPolicy = pullPolicy
	return b
}
