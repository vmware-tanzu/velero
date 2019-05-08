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

package backup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

// Builder is a helper for concisely constructing Backup API objects.
type Builder struct {
	backup velerov1api.Backup
}

// NewBuilder returns a Builder for a Backup with no namespace/name.
func NewBuilder() *Builder {
	return NewNamedBuilder("", "")
}

// NewNamedBuilder returns a Builder for a Backup with the specified namespace
// and name.
func NewNamedBuilder(namespace, name string) *Builder {
	return &Builder{
		backup: velerov1api.Backup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "Backup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

// Build returns the built Backup API object.
func (b *Builder) Build() *velerov1api.Backup {
	return &b.backup
}

// Namespace sets the Backup's namespace.
func (b *Builder) Namespace(namespace string) *Builder {
	b.backup.Namespace = namespace
	return b
}

// Name sets the Backup's name.
func (b *Builder) Name(name string) *Builder {
	b.backup.Name = name
	return b
}

// IncludedNamespaces sets the Backup's included namespaces.
func (b *Builder) IncludedNamespaces(namespaces ...string) *Builder {
	b.backup.Spec.IncludedNamespaces = namespaces
	return b
}

// ExcludedNamespaces sets the Backup's excluded namespaces.
func (b *Builder) ExcludedNamespaces(namespaces ...string) *Builder {
	b.backup.Spec.ExcludedNamespaces = namespaces
	return b
}

// IncludedResources sets the Backup's included resources.
func (b *Builder) IncludedResources(resources ...string) *Builder {
	b.backup.Spec.IncludedResources = resources
	return b
}

// ExcludedResources sets the Backup's excluded resources.
func (b *Builder) ExcludedResources(resources ...string) *Builder {
	b.backup.Spec.ExcludedResources = resources
	return b
}

// IncludeClusterResources sets the Backup's "include cluster resources" flag.
func (b *Builder) IncludeClusterResources(val bool) *Builder {
	b.backup.Spec.IncludeClusterResources = &val
	return b
}

// LabelSelector sets the Backup's label selector.
func (b *Builder) LabelSelector(selector *metav1.LabelSelector) *Builder {
	b.backup.Spec.LabelSelector = selector
	return b
}

// SnapshotVolumes sets the Backup's "snapshot volumes" flag.
func (b *Builder) SnapshotVolumes(val bool) *Builder {
	b.backup.Spec.SnapshotVolumes = &val
	return b
}
