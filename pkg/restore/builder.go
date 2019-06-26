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

package restore

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

// Builder is a helper for concisely constructing Restore API objects.
type Builder struct {
	restore velerov1api.Restore
}

// NewBuilder returns a Builder for a Restore with no namespace/name.
func NewBuilder() *Builder {
	return NewNamedBuilder("", "")
}

// NewNamedBuilder returns a Builder for a Restore with the specified namespace
// and name.
func NewNamedBuilder(namespace, name string) *Builder {
	return &Builder{
		restore: velerov1api.Restore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "Restore",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

// Restore returns the built Restore API object.
func (b *Builder) Restore() *velerov1api.Restore {
	return &b.restore
}

// Backup sets the Restore's backup name.
func (b *Builder) Backup(name string) *Builder {
	b.restore.Spec.BackupName = name
	return b
}

// IncludedNamespaces sets the Restore's included namespaces.
func (b *Builder) IncludedNamespaces(namespaces ...string) *Builder {
	b.restore.Spec.IncludedNamespaces = namespaces
	return b
}

// ExcludedNamespaces sets the Restore's excluded namespaces.
func (b *Builder) ExcludedNamespaces(namespaces ...string) *Builder {
	b.restore.Spec.ExcludedNamespaces = namespaces
	return b
}

// IncludedResources sets the Restore's included resources.
func (b *Builder) IncludedResources(resources ...string) *Builder {
	b.restore.Spec.IncludedResources = resources
	return b
}

// ExcludedResources sets the Restore's excluded resources.
func (b *Builder) ExcludedResources(resources ...string) *Builder {
	b.restore.Spec.ExcludedResources = resources
	return b
}

// IncludeClusterResources sets the Restore's "include cluster resources" flag.
func (b *Builder) IncludeClusterResources(val bool) *Builder {
	b.restore.Spec.IncludeClusterResources = &val
	return b
}

// LabelSelector sets the Restore's label selector.
func (b *Builder) LabelSelector(selector *metav1.LabelSelector) *Builder {
	b.restore.Spec.LabelSelector = selector
	return b
}

// NamespaceMappings sets the Restore's namespace mappings.
func (b *Builder) NamespaceMappings(mapping ...string) *Builder {
	if b.restore.Spec.NamespaceMapping == nil {
		b.restore.Spec.NamespaceMapping = make(map[string]string)
	}

	if len(mapping)%2 != 0 {
		panic("mapping must contain an even number of values")
	}

	for i := 0; i < len(mapping); i += 2 {
		b.restore.Spec.NamespaceMapping[mapping[i]] = mapping[i+1]
	}

	return b
}
