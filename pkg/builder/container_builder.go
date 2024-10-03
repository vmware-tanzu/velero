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
	"encoding/json"
	"strings"

	corev1api "k8s.io/api/core/v1"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
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
// image that includes the entire string except the registry name, and transforms the combined
// string into a RFC-1123 compatible name.
func getName(image string) string {
	slashIndex := strings.Index(image, "/")
	slashCount := 0
	if slashIndex >= 0 {
		slashCount = strings.Count(image[slashIndex:], "/")
	}

	start := 0
	if slashCount > 1 || slashIndex == 0 {
		// always start after the first slash when there is a registry name
		// or if the string starts with a slash.
		start = slashIndex + 1
	}

	// If the image spec is by digest, remove the digest.
	// If it is by tag, remove the tag.
	// Otherwise (implicit :latest) leave it alone.
	end := len(image)
	atIndex := strings.LastIndex(image, "@")
	if atIndex > 0 {
		end = atIndex
	} else {
		colonIndex := strings.LastIndex(image, ":")
		if colonIndex > 0 {
			end = colonIndex
		}
	}

	// https://github.com/distribution/distribution/blob/main/docs/spec/api.md#overview
	// valid repository names match the regex [a-z0-9]+(?:[._-][a-z0-9]+)*
	// image repository names can container [._] but [._] are not allowed in RFC-1123 labels.
	// replace '/', '_' and '.' with '-'
	re := strings.NewReplacer("/", "-",
		"_", "-",
		".", "-")
	return re.Replace(image[start:end])
}

// Result returns the built Container.
func (b *ContainerBuilder) Result() *corev1api.Container {
	return b.object
}

// ResultRawExtension returns the Container as runtime.RawExtension.
func (b *ContainerBuilder) ResultRawExtension() apimachineryRuntime.RawExtension {
	result, err := json.Marshal(b.object)
	if err != nil {
		return apimachineryRuntime.RawExtension{}
	}
	return apimachineryRuntime.RawExtension{
		Raw: result,
	}
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
