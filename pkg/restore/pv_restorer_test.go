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
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/backup"
	cloudprovidermocks "github.com/heptio/velero/pkg/cloudprovider/mocks"
	"github.com/heptio/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions"
	"github.com/heptio/velero/pkg/plugin/velero"
	velerotest "github.com/heptio/velero/pkg/util/test"
	"github.com/heptio/velero/pkg/volume"
)

func defaultBackup() *backup.Builder {
	return backup.NewNamedBuilder(api.DefaultNamespace, "backup-1")
}

func TestExecutePVAction_NoSnapshotRestores(t *testing.T) {
	tests := []struct {
		name            string
		obj             *unstructured.Unstructured
		restore         *api.Restore
		backup          *api.Backup
		volumeSnapshots []*volume.Snapshot
		locations       []*api.VolumeSnapshotLocation
		expectedErr     bool
		expectedRes     *unstructured.Unstructured
	}{
		{
			name:        "no name should error",
			obj:         NewTestUnstructured().WithMetadata().Unstructured,
			restore:     velerotest.NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("pv-1").Unstructured,
			restore:     velerotest.NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "ensure spec.claimRef is deleted",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "someOtherField").Unstructured,
			restore:     velerotest.NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:      defaultBackup().Phase(api.BackupPhaseInProgress).Backup(),
			expectedRes: NewTestUnstructured().WithAnnotations("a", "b").WithName("pv-1").WithSpec("someOtherField").Unstructured,
		},
		{
			name:        "ensure spec.storageClassName is retained",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("storageClassName", "someOtherField").Unstructured,
			restore:     velerotest.NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:      defaultBackup().Phase(api.BackupPhaseInProgress).Backup(),
			expectedRes: NewTestUnstructured().WithAnnotations("a", "b").WithName("pv-1").WithSpec("storageClassName", "someOtherField").Unstructured,
		},
		{
			name:        "if backup.spec.snapshotVolumes is false, ignore restore.spec.restorePVs and return early",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "storageClassName", "someOtherField").Unstructured,
			restore:     velerotest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      defaultBackup().Phase(api.BackupPhaseInProgress).SnapshotVolumes(false).Backup(),
			expectedRes: NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("storageClassName", "someOtherField").Unstructured,
		},
		{
			name:    "restore.spec.restorePVs=false, return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: velerotest.NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:  defaultBackup().Phase(api.BackupPhaseInProgress).Backup(),
			volumeSnapshots: []*volume.Snapshot{
				newSnapshot("pv-1", "loc-1", "gp", "az-1", "snap-1", 1000),
			},
			locations: []*api.VolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-1").VolumeSnapshotLocation,
			},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:    "volumeSnapshots is empty: return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: velerotest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:  defaultBackup().Backup(),
			locations: []*api.VolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-1").VolumeSnapshotLocation,
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-2").VolumeSnapshotLocation,
			},
			volumeSnapshots: []*volume.Snapshot{},
			expectedRes:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:    "volumeSnapshots doesn't have a snapshot for PV: return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: velerotest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:  defaultBackup().Backup(),
			locations: []*api.VolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-1").VolumeSnapshotLocation,
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-2").VolumeSnapshotLocation,
			},
			volumeSnapshots: []*volume.Snapshot{
				newSnapshot("non-matching-pv-1", "loc-1", "type-1", "az-1", "snap-1", 1),
				newSnapshot("non-matching-pv-2", "loc-2", "type-2", "az-2", "snap-2", 2),
			},
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				client                   = fake.NewSimpleClientset()
				snapshotLocationInformer = informers.NewSharedInformerFactory(client, 0).Velero().V1().VolumeSnapshotLocations()
			)

			r := &pvRestorer{
				logger:                 velerotest.NewLogger(),
				restorePVs:             tc.restore.Spec.RestorePVs,
				snapshotLocationLister: snapshotLocationInformer.Lister(),
			}
			if tc.backup != nil {
				r.backup = tc.backup
				r.snapshotVolumes = tc.backup.Spec.SnapshotVolumes
			}

			for _, loc := range tc.locations {
				require.NoError(t, snapshotLocationInformer.Informer().GetStore().Add(loc))
			}

			res, err := r.executePVAction(tc.obj)
			switch tc.expectedErr {
			case true:
				assert.Nil(t, res)
				assert.NotNil(t, err)
			case false:
				assert.Equal(t, tc.expectedRes, res)
				assert.Nil(t, err)
			}
		})
	}
}

func TestExecutePVAction_SnapshotRestores(t *testing.T) {
	tests := []struct {
		name               string
		obj                *unstructured.Unstructured
		restore            *api.Restore
		backup             *api.Backup
		volumeSnapshots    []*volume.Snapshot
		locations          []*api.VolumeSnapshotLocation
		expectedProvider   string
		expectedSnapshotID string
		expectedVolumeType string
		expectedVolumeAZ   string
		expectedVolumeIOPS *int64
		expectedSnapshot   *volume.Snapshot
	}{
		{
			name:    "backup with a matching volume.Snapshot for PV executes restore",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: velerotest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:  defaultBackup().Backup(),
			locations: []*api.VolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-1").WithProvider("provider-1").VolumeSnapshotLocation,
				velerotest.NewTestVolumeSnapshotLocation().WithName("loc-2").WithProvider("provider-2").VolumeSnapshotLocation,
			},
			volumeSnapshots: []*volume.Snapshot{
				newSnapshot("pv-1", "loc-1", "type-1", "az-1", "snap-1", 1),
				newSnapshot("pv-2", "loc-2", "type-2", "az-2", "snap-2", 2),
			},
			expectedProvider:   "provider-1",
			expectedSnapshotID: "snap-1",
			expectedVolumeType: "type-1",
			expectedVolumeAZ:   "az-1",
			expectedVolumeIOPS: int64Ptr(1),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				volumeSnapshotter       = new(cloudprovidermocks.VolumeSnapshotter)
				volumeSnapshotterGetter = providerToVolumeSnapshotterMap(map[string]velero.VolumeSnapshotter{
					tc.expectedProvider: volumeSnapshotter,
				})
				locationsInformer = informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Velero().V1().VolumeSnapshotLocations()
			)

			for _, loc := range tc.locations {
				require.NoError(t, locationsInformer.Informer().GetStore().Add(loc))
			}

			r := &pvRestorer{
				logger:                  velerotest.NewLogger(),
				backup:                  tc.backup,
				volumeSnapshots:         tc.volumeSnapshots,
				snapshotLocationLister:  locationsInformer.Lister(),
				volumeSnapshotterGetter: volumeSnapshotterGetter,
			}

			volumeSnapshotter.On("Init", mock.Anything).Return(nil)
			volumeSnapshotter.On("CreateVolumeFromSnapshot", tc.expectedSnapshotID, tc.expectedVolumeType, tc.expectedVolumeAZ, tc.expectedVolumeIOPS).Return("volume-1", nil)
			volumeSnapshotter.On("SetVolumeID", tc.obj, "volume-1").Return(tc.obj, nil)

			_, err := r.executePVAction(tc.obj)
			assert.NoError(t, err)

			volumeSnapshotter.AssertExpectations(t)
		})
	}
}

type providerToVolumeSnapshotterMap map[string]velero.VolumeSnapshotter

func (g providerToVolumeSnapshotterMap) GetVolumeSnapshotter(provider string) (velero.VolumeSnapshotter, error) {
	if bs, ok := g[provider]; !ok {
		return nil, errors.New("volume snapshotter not found for provider")
	} else {
		return bs, nil
	}
}

func newSnapshot(pvName, location, volumeType, volumeAZ, snapshotID string, volumeIOPS int64) *volume.Snapshot {
	return &volume.Snapshot{
		Spec: volume.SnapshotSpec{
			PersistentVolumeName: pvName,
			Location:             location,
			VolumeType:           volumeType,
			VolumeAZ:             volumeAZ,
			VolumeIOPS:           &volumeIOPS,
		},
		Status: volume.SnapshotStatus{
			ProviderSnapshotID: snapshotID,
		},
	}
}

func int64Ptr(val int) *int64 {
	r := int64(val)
	return &r
}
