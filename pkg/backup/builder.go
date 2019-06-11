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
	"time"

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

// Backup returns the built Backup API object.
func (b *Builder) Backup() *velerov1api.Backup {
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

// Labels sets the Backup's labels.
func (b *Builder) Labels(vals ...string) *Builder {
	if b.backup.Labels == nil {
		b.backup.Labels = map[string]string{}
	}

	// if we don't have an even number of values, e.g. a key and a value
	// for each pair, add an empty-string value at the end to serve as
	// the default value for the last key.
	if len(vals)%2 != 0 {
		vals = append(vals, "")
	}

	for i := 0; i < len(vals); i += 2 {
		b.backup.Labels[vals[i]] = vals[i+1]
	}
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

// Phase sets the Backup's phase.
func (b *Builder) Phase(phase velerov1api.BackupPhase) *Builder {
	b.backup.Status.Phase = phase
	return b
}

// StorageLocation sets the Backup's storage location.
func (b *Builder) StorageLocation(location string) *Builder {
	b.backup.Spec.StorageLocation = location
	return b
}

// VolumeSnapshotLocations sets the Backup's volume snapshot locations.
func (b *Builder) VolumeSnapshotLocations(locations ...string) *Builder {
	b.backup.Spec.VolumeSnapshotLocations = locations
	return b
}

// TTL sets the Backup's TTL.
func (b *Builder) TTL(ttl time.Duration) *Builder {
	b.backup.Spec.TTL.Duration = ttl
	return b
}

// Expiration sets the Backup's expiration.
func (b *Builder) Expiration(val time.Time) *Builder {
	b.backup.Status.Expiration.Time = val
	return b
}

// StartTimestamp sets the Backup's start timestamp.
func (b *Builder) StartTimestamp(val time.Time) *Builder {
	b.backup.Status.StartTimestamp.Time = val
	return b
}

// NoTypeMeta removes the type meta from the Backup.
func (b *Builder) NoTypeMeta() *Builder {
	b.backup.TypeMeta = metav1.TypeMeta{}
	return b
}

// Hooks sets the Backup's hooks.
func (b *Builder) Hooks(hooks velerov1api.BackupHooks) *Builder {
	b.backup.Spec.Hooks = hooks
	return b
}
