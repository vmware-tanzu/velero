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

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	providermocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

func defaultBackup() *builder.BackupBuilder {
	return builder.ForBackup(api.DefaultNamespace, "backup-1")
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
			restore:     builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedErr: true,
		},
		{
			name:        "ensure spec.storageClassName is retained",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("storageClassName", "someOtherField").Unstructured,
			restore:     builder.ForRestore(api.DefaultNamespace, "").RestorePVs(false).Result(),
			backup:      defaultBackup().Phase(api.BackupPhaseInProgress).Result(),
			expectedRes: NewTestUnstructured().WithAnnotations("a", "b").WithName("pv-1").WithSpec("storageClassName", "someOtherField").Unstructured,
		},
		{
			name:        "if backup.spec.snapshotVolumes is false, ignore restore.spec.restorePVs and return early",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "storageClassName", "someOtherField").Unstructured,
			restore:     builder.ForRestore(api.DefaultNamespace, "").RestorePVs(true).Result(),
			backup:      defaultBackup().Phase(api.BackupPhaseInProgress).SnapshotVolumes(false).Result(),
			expectedRes: NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "storageClassName", "someOtherField").Unstructured,
		},
		{
			name:    "restore.spec.restorePVs=false, return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: builder.ForRestore(api.DefaultNamespace, "").RestorePVs(false).Result(),
			backup:  defaultBackup().Phase(api.BackupPhaseInProgress).Result(),
			volumeSnapshots: []*volume.Snapshot{
				newSnapshot("pv-1", "loc-1", "gp", "az-1", "snap-1", 1000),
			},
			locations: []*api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-1").Result(),
			},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:    "volumeSnapshots is empty: return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: builder.ForRestore(api.DefaultNamespace, "").RestorePVs(true).Result(),
			backup:  defaultBackup().Result(),
			locations: []*api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-1").Result(),
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-2").Result(),
			},
			volumeSnapshots: []*volume.Snapshot{},
			expectedRes:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:    "volumeSnapshots doesn't have a snapshot for PV: return early",
			obj:     NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore: builder.ForRestore(api.DefaultNamespace, "").RestorePVs(true).Result(),
			backup:  defaultBackup().Result(),
			locations: []*api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-1").Result(),
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-2").Result(),
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
			restore: builder.ForRestore(api.DefaultNamespace, "").RestorePVs(true).Result(),
			backup:  defaultBackup().Result(),
			locations: []*api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-1").Provider("provider-1").Result(),
				builder.ForVolumeSnapshotLocation(api.DefaultNamespace, "loc-2").Provider("provider-2").Result(),
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
				volumeSnapshotter       = new(providermocks.VolumeSnapshotter)
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

type testUnstructured struct {
	*unstructured.Unstructured
}

func NewTestUnstructured() *testUnstructured {
	obj := &testUnstructured{
		Unstructured: &unstructured.Unstructured{
			Object: make(map[string]interface{}),
		},
	}

	return obj
}

func (obj *testUnstructured) WithMetadata(fields ...string) *testUnstructured {
	return obj.withMap("metadata", fields...)
}

func (obj *testUnstructured) WithSpec(fields ...string) *testUnstructured {
	if _, found := obj.Object["spec"]; found {
		panic("spec already set - you probably didn't mean to do this twice!")
	}
	return obj.withMap("spec", fields...)
}

func (obj *testUnstructured) WithStatus(fields ...string) *testUnstructured {
	return obj.withMap("status", fields...)
}

func (obj *testUnstructured) WithMetadataField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("metadata", field, value)
}

func (obj *testUnstructured) WithSpecField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("spec", field, value)
}

func (obj *testUnstructured) WithStatusField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("status", field, value)
}

func (obj *testUnstructured) WithAnnotations(fields ...string) *testUnstructured {
	vals := map[string]string{}
	for _, field := range fields {
		vals[field] = "foo"
	}

	return obj.WithAnnotationValues(vals)
}

func (obj *testUnstructured) WithAnnotationValues(fieldVals map[string]string) *testUnstructured {
	annotations := make(map[string]interface{})
	for field, val := range fieldVals {
		annotations[field] = val
	}

	obj = obj.WithMetadataField("annotations", annotations)

	return obj
}

func (obj *testUnstructured) WithName(name string) *testUnstructured {
	return obj.WithMetadataField("name", name)
}

func (obj *testUnstructured) withMap(name string, fields ...string) *testUnstructured {
	m := make(map[string]interface{})
	obj.Object[name] = m

	for _, field := range fields {
		m[field] = "foo"
	}

	return obj
}

func (obj *testUnstructured) withMapEntry(mapName, field string, value interface{}) *testUnstructured {
	var m map[string]interface{}

	if res, ok := obj.Unstructured.Object[mapName]; !ok {
		m = make(map[string]interface{})
		obj.Unstructured.Object[mapName] = m
	} else {
		m = res.(map[string]interface{})
	}

	m[field] = value

	return obj
}
