/*
Copyright The Velero Contributors.

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

package csi

import (
	"fmt"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
)

// ResetVolumeSnapshotContent make changes to the volumesnapshot content before it's persisted
// It will move the snapshot Handle to the source to avoid the snapshot-controller creating a snapshot when it's
// synced by the backup sync controller.
// It will return an error if the snapshot handle is not set, which should not happen when this func is called.
func ResetVolumeSnapshotContent(snapCont *snapshotv1api.VolumeSnapshotContent) error {
	if snapCont.Status != nil && snapCont.Status.SnapshotHandle != nil && len(*snapCont.Status.SnapshotHandle) > 0 {
		v := *snapCont.Status.SnapshotHandle
		snapCont.Spec.Source = snapshotv1api.VolumeSnapshotContentSource{
			SnapshotHandle: &v,
		}
	} else {
		return fmt.Errorf("the volumesnapshotcontent '%s' does not have snapshothandle set", snapCont.Name)
	}

	// set the VolumeSnapshotRef to non-existing one to bypass the validation webhook
	snapCont.Spec.VolumeSnapshotRef = corev1.ObjectReference{
		Namespace: fmt.Sprintf("ns-%s", snapCont.UID),
		Name:      fmt.Sprintf("name-%s", snapCont.UID),
	}
	return nil
}
