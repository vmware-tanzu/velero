/*
Copyright 2023 the Velero contributors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PodVolumeRestoreBuilder builds PodVolumeRestore objects.
type PodVolumeRestoreBuilder struct {
	object *velerov1api.PodVolumeRestore
}

// ForPodVolumeRestore is the constructor for a PodVolumeRestoreBuilder.
func ForPodVolumeRestore(ns, name string) *PodVolumeRestoreBuilder {
	return &PodVolumeRestoreBuilder{
		object: &velerov1api.PodVolumeRestore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "PodVolumeRestore",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) Result() *velerov1api.PodVolumeRestore {
	return b.object
}

// ObjectMeta applies functional options to the PodVolumeRestore's ObjectMeta.
func (b *PodVolumeRestoreBuilder) ObjectMeta(opts ...ObjectMetaOpt) *PodVolumeRestoreBuilder {
	for _, opt := range opts {
		opt(b.object)
	}
	return b
}

// Phase sets the PodVolumeRestore's phase.
func (b *PodVolumeRestoreBuilder) Phase(phase velerov1api.PodVolumeRestorePhase) *PodVolumeRestoreBuilder {
	b.object.Status.Phase = phase
	return b
}

// BackupStorageLocation sets the PodVolumeRestore's backup storage location.
func (b *PodVolumeRestoreBuilder) BackupStorageLocation(name string) *PodVolumeRestoreBuilder {
	b.object.Spec.BackupStorageLocation = name
	return b
}

// SnapshotID sets the PodVolumeRestore's snapshot ID.
func (b *PodVolumeRestoreBuilder) SnapshotID(snapshotID string) *PodVolumeRestoreBuilder {
	b.object.Spec.SnapshotID = snapshotID
	return b
}

// PodName sets the name of the pod associated with this PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) PodName(name string) *PodVolumeRestoreBuilder {
	b.object.Spec.Pod.Name = name
	return b
}

// PodNamespace sets the name of the pod associated with this PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) PodNamespace(ns string) *PodVolumeRestoreBuilder {
	b.object.Spec.Pod.Namespace = ns
	return b
}

// Volume sets the name of the volume associated with this PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) Volume(volume string) *PodVolumeRestoreBuilder {
	b.object.Spec.Volume = volume
	return b
}

// UploaderType sets the type of uploader to use for this PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) UploaderType(uploaderType string) *PodVolumeRestoreBuilder {
	b.object.Spec.UploaderType = uploaderType
	return b
}

// OwnerReference sets the OwnerReference for this PodVolumeRestore.
func (b *PodVolumeRestoreBuilder) OwnerReference(ownerRef []metav1.OwnerReference) *PodVolumeRestoreBuilder {
	b.object.OwnerReferences = ownerRef
	return b
}
