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
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeSnapshotClassBuilder builds VolumeSnapshotClass objects.
type VolumeSnapshotClassBuilder struct {
	object *snapshotv1api.VolumeSnapshotClass
}

// ForVolumeSnapshotClass is the constructor of VolumeSnapshotClassBuilder.
func ForVolumeSnapshotClass(name string) *VolumeSnapshotClassBuilder {
	return &VolumeSnapshotClassBuilder{
		object: &snapshotv1api.VolumeSnapshotClass{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VolumeSnapshotClass",
				APIVersion: snapshotv1api.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Result returns the built VolumeSnapshotClass.
func (b *VolumeSnapshotClassBuilder) Result() *snapshotv1api.VolumeSnapshotClass {
	return b.object
}

// Driver sets the driver of built VolumeSnapshotClass.
func (b *VolumeSnapshotClassBuilder) Driver(driver string) *VolumeSnapshotClassBuilder {
	b.object.Driver = driver
	return b
}

// ObjectMeta applies functional options to the VolumeSnapshotClass's ObjectMeta.
func (b *VolumeSnapshotClassBuilder) ObjectMeta(opts ...ObjectMetaOpt) *VolumeSnapshotClassBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
