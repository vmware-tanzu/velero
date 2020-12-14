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
// image that includes its reposiroty name, and transforms the combined
// string into a DNS-1123 compatible name.
func getName(image string) string {
	slashIndex := strings.Index(image, "/")
	slashCount := strings.Count(image[slashIndex:], "/")
	colonIndex := strings.LastIndex(image, ":")

	// this removes the registry name when there is one, but keeps the repository name
	start := 0
	if slashCount == 1 {
		start = 0
	} else {
		// this will be the first character after the first found slash
		start = slashIndex + 1
	}

	// this removes the tag
	end := len(image)
	if colonIndex > 0 {
		end = colonIndex
	}

	return strings.Replace(image[start:end], "/", "-", 1) // this makes it DNS-1123 compatible
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

// SecurityContext sets the container's SecurityContext.
func (b *ContainerBuilder) SecurityContext(securityContext *corev1api.SecurityContext) *ContainerBuilder {
	b.object.SecurityContext = securityContext
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

func (b *ContainerBuilder) Command(command []string) *ContainerBuilder {
	if b.object.Command == nil {
		b.object.Command = []string{}
	}

	b.object.Command = append(b.object.Command, command...)

	return b
}
