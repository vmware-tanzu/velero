/*
Copyright 2020 the Velero contributors.

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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

/*

Example usage:

var backup = builder.ForBackup("velero", "backup-1").
	ObjectMeta(
		builder.WithLabels("foo", "bar"),
		builder.WithClusterName("cluster-1"),
	).
	SnapshotVolumes(true).
	Result()

*/

// BackupBuilder builds Backup objects.
type BackupBuilder struct {
	object *velerov1api.Backup
}

// ForBackup is the constructor for a BackupBuilder.
func ForBackup(ns, name string) *BackupBuilder {
	return &BackupBuilder{
		object: &velerov1api.Backup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "Backup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built Backup.
func (b *BackupBuilder) Result() *velerov1api.Backup {
	return b.object
}

// ObjectMeta applies functional options to the Backup's ObjectMeta.
func (b *BackupBuilder) ObjectMeta(opts ...ObjectMetaOpt) *BackupBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// FromSchedule sets the Backup's spec and labels from the Schedule template
func (b *BackupBuilder) FromSchedule(schedule *velerov1api.Schedule) *BackupBuilder {
	labels := schedule.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[velerov1api.ScheduleNameLabel] = schedule.Name

	b.object.Spec = schedule.Spec.Template
	b.ObjectMeta(WithLabelsMap(labels))

	if schedule.Annotations != nil {
		b.ObjectMeta(WithAnnotationsMap(schedule.Annotations))
	}

	if boolptr.IsSetToTrue(schedule.Spec.UseOwnerReferencesInBackup) {
		b.object.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "Schedule",
				Name:       schedule.Name,
				UID:        schedule.UID,
				Controller: boolptr.True(),
			},
		})
	}

	return b
}

// IncludedNamespaces sets the Backup's included namespaces.
func (b *BackupBuilder) IncludedNamespaces(namespaces ...string) *BackupBuilder {
	b.object.Spec.IncludedNamespaces = namespaces
	return b
}

// ExcludedNamespaces sets the Backup's excluded namespaces.
func (b *BackupBuilder) ExcludedNamespaces(namespaces ...string) *BackupBuilder {
	b.object.Spec.ExcludedNamespaces = namespaces
	return b
}

// IncludedResources sets the Backup's included resources.
func (b *BackupBuilder) IncludedResources(resources ...string) *BackupBuilder {
	b.object.Spec.IncludedResources = resources
	return b
}

// ExcludedResources sets the Backup's excluded resources.
func (b *BackupBuilder) ExcludedResources(resources ...string) *BackupBuilder {
	b.object.Spec.ExcludedResources = resources
	return b
}

// IncludeClusterResources sets the Backup's "include cluster resources" flag.
func (b *BackupBuilder) IncludeClusterResources(val bool) *BackupBuilder {
	b.object.Spec.IncludeClusterResources = &val
	return b
}

// LabelSelector sets the Backup's label selector.
func (b *BackupBuilder) LabelSelector(selector *metav1.LabelSelector) *BackupBuilder {
	b.object.Spec.LabelSelector = selector
	return b
}

// SnapshotVolumes sets the Backup's "snapshot volumes" flag.
func (b *BackupBuilder) SnapshotVolumes(val bool) *BackupBuilder {
	b.object.Spec.SnapshotVolumes = &val
	return b
}

// DefaultVolumesToRestic sets the Backup's "DefaultVolumesToRestic" flag.
func (b *BackupBuilder) DefaultVolumesToRestic(val bool) *BackupBuilder {
	b.object.Spec.DefaultVolumesToRestic = &val
	return b
}

// Phase sets the Backup's phase.
func (b *BackupBuilder) Phase(phase velerov1api.BackupPhase) *BackupBuilder {
	b.object.Status.Phase = phase
	return b
}

// StorageLocation sets the Backup's storage location.
func (b *BackupBuilder) StorageLocation(location string) *BackupBuilder {
	b.object.Spec.StorageLocation = location
	return b
}

// VolumeSnapshotLocations sets the Backup's volume snapshot locations.
func (b *BackupBuilder) VolumeSnapshotLocations(locations ...string) *BackupBuilder {
	b.object.Spec.VolumeSnapshotLocations = locations
	return b
}

// TTL sets the Backup's TTL.
func (b *BackupBuilder) TTL(ttl time.Duration) *BackupBuilder {
	b.object.Spec.TTL.Duration = ttl
	return b
}

// Expiration sets the Backup's expiration.
func (b *BackupBuilder) Expiration(val time.Time) *BackupBuilder {
	b.object.Status.Expiration = &metav1.Time{Time: val}
	return b
}

// StartTimestamp sets the Backup's start timestamp.
func (b *BackupBuilder) StartTimestamp(val time.Time) *BackupBuilder {
	b.object.Status.StartTimestamp = &metav1.Time{Time: val}
	return b
}

// CompletionTimestamp sets the Backup's completion timestamp.
func (b *BackupBuilder) CompletionTimestamp(val time.Time) *BackupBuilder {
	b.object.Status.CompletionTimestamp = &metav1.Time{Time: val}
	return b
}

// Hooks sets the Backup's hooks.
func (b *BackupBuilder) Hooks(hooks velerov1api.BackupHooks) *BackupBuilder {
	b.object.Spec.Hooks = hooks
	return b
}

// OrderedResources sets the Backup's OrderedResources
func (b *BackupBuilder) OrderedResources(orders map[string]string) *BackupBuilder {
	b.object.Spec.OrderedResources = orders
	return b
}
