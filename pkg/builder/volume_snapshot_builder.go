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
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeSnapshotBuilder builds VolumeSnapshot objects.
type VolumeSnapshotBuilder struct {
	object *snapshotv1api.VolumeSnapshot
}

// ForVolumeSnapshot is the constructor for VolumeSnapshotBuilder.
func ForVolumeSnapshot(ns, name string) *VolumeSnapshotBuilder {
	return &VolumeSnapshotBuilder{
		object: &snapshotv1api.VolumeSnapshot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: snapshotv1api.SchemeGroupVersion.String(),
				Kind:       "VolumeSnapshot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
		},
	}
}

// ObjectMeta applies functional options to the VolumeSnapshot's ObjectMeta.
func (v *VolumeSnapshotBuilder) ObjectMeta(opts ...ObjectMetaOpt) *VolumeSnapshotBuilder {
	for _, opt := range opts {
		opt(v.object)
	}

	return v
}

// Result return the built VolumeSnapshot.
func (v *VolumeSnapshotBuilder) Result() *snapshotv1api.VolumeSnapshot {
	return v.object
}

// Status init the built VolumeSnapshot's status.
func (v *VolumeSnapshotBuilder) Status() *VolumeSnapshotBuilder {
	v.object.Status = &snapshotv1api.VolumeSnapshotStatus{}
	return v
}

// BoundVolumeSnapshotContentName set built VolumeSnapshot's status BoundVolumeSnapshotContentName field.
func (v *VolumeSnapshotBuilder) BoundVolumeSnapshotContentName(vscName string) *VolumeSnapshotBuilder {
	v.object.Status.BoundVolumeSnapshotContentName = &vscName
	return v
}
