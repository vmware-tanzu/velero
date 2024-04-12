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

package csi

import (
	"testing"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testPVC       = "test-pvc"
	testSnapClass = "snap-class"
	randText      = "DEADFEED"
)

func TestResetVolumeSnapshotSpecForRestore(t *testing.T) {
	testCases := []struct {
		name    string
		vs      snapshotv1api.VolumeSnapshot
		vscName string
	}{
		{
			name: "should reset spec as expected",
			vs: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-ns",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					Source: snapshotv1api.VolumeSnapshotSource{
						PersistentVolumeClaimName: &testPVC,
					},
					VolumeSnapshotClassName: &testSnapClass,
				},
			},
			vscName: "test-vsc",
		},
		{
			name: "should reset spec and overwriting value for Source.VolumeSnapshotContentName",
			vs: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-ns",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					Source: snapshotv1api.VolumeSnapshotSource{
						VolumeSnapshotContentName: &randText,
					},
					VolumeSnapshotClassName: &testSnapClass,
				},
			},
			vscName: "test-vsc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.vs.DeepCopy()
			resetVolumeSnapshotSpecForRestore(&tc.vs, &tc.vscName)

			assert.Equalf(t, tc.vs.Name, before.Name, "unexpected change to Object.Name, Want: %s; Got %s", tc.name, before.Name, tc.vs.Name)
			assert.Equal(t, tc.vs.Namespace, before.Namespace, "unexpected change to Object.Namespace, Want: %s; Got %s", tc.name, before.Namespace, tc.vs.Namespace)
			assert.NotNil(t, tc.vs.Spec.Source)
			assert.Nil(t, tc.vs.Spec.Source.PersistentVolumeClaimName)
			assert.NotNil(t, tc.vs.Spec.Source.VolumeSnapshotContentName)
			assert.Equal(t, *tc.vs.Spec.Source.VolumeSnapshotContentName, tc.vscName)
			assert.Equal(t, *tc.vs.Spec.VolumeSnapshotClassName, *before.Spec.VolumeSnapshotClassName, "unexpected value for Spec.VolumeSnapshotClassName, Want: %s, Got: %s",
				*tc.vs.Spec.VolumeSnapshotClassName, *before.Spec.VolumeSnapshotClassName)
			assert.Nil(t, tc.vs.Status)
		})
	}
}
