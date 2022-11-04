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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeSnapshotContentBuilder builds VolumeSnapshotContent object.
type VolumeSnapshotContentBuilder struct {
	object *snapshotv1api.VolumeSnapshotContent
}

// ForVolumeSnapshotContent is the constructor of VolumeSnapshotContentBuilder.
func ForVolumeSnapshotContent(name string) *VolumeSnapshotContentBuilder {
	return &VolumeSnapshotContentBuilder{
		object: &snapshotv1api.VolumeSnapshotContent{
			TypeMeta: metav1.TypeMeta{
				APIVersion: snapshotv1api.SchemeGroupVersion.String(),
				Kind:       "VolumeSnapshotContent",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Result returns the built VolumeSnapshotContent.
func (v *VolumeSnapshotContentBuilder) Result() *snapshotv1api.VolumeSnapshotContent {
	return v.object
}

// Status initiates VolumeSnapshotContent's status.
func (v *VolumeSnapshotContentBuilder) Status() *VolumeSnapshotContentBuilder {
	v.object.Status = &snapshotv1api.VolumeSnapshotContentStatus{}
	return v
}

// DeletionPolicy sets built VolumeSnapshotContent's spec.DeletionPolicy value.
func (v *VolumeSnapshotContentBuilder) DeletionPolicy(policy snapshotv1api.DeletionPolicy) *VolumeSnapshotContentBuilder {
	v.object.Spec.DeletionPolicy = policy
	return v
}

func (v *VolumeSnapshotContentBuilder) VolumeSnapshotRef(namespace, name string) *VolumeSnapshotContentBuilder {
	v.object.Spec.VolumeSnapshotRef = v1.ObjectReference{
		APIVersion: "snapshot.storage.k8s.io/v1",
		Kind:       "VolumeSnapshot",
		Namespace:  namespace,
		Name:       name,
	}
	return v
}
