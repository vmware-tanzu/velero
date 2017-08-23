/*
Copyright 2017 Heptio Inc.

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

package restorers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	. "github.com/heptio/ark/pkg/util/test"
)

func TestPVRestorerPrepare(t *testing.T) {
	iops := int64(1000)

	tests := []struct {
		name              string
		obj               runtime.Unstructured
		restore           *api.Restore
		backup            *api.Backup
		volumeMap         map[api.VolumeBackupInfo]string
		noSnapshotService bool
		expectedWarn      bool
		expectedErr       bool
		expectedRes       runtime.Unstructured
	}{
		{
			name:        "no name should error",
			obj:         NewTestUnstructured().WithMetadata().Unstructured,
			restore:     NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("pv-1").Unstructured,
			restore:     NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "when RestorePVs=false, should not error if there is no PV->BackupInfo map",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{}},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:        "when RestorePVs=true, return without error if there is no PV->BackupInfo map",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{}},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
		},
		{
			name:        "when RestorePVs=true, error if there is PV->BackupInfo map but no entry for this PV",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"another-pv": &api.VolumeBackupInfo{}}}},
			expectedErr: true,
		},
		{
			name: "claimRef and storageClassName (only) should be cleared from spec",
			obj: NewTestUnstructured().
				WithName("pv-1").
				WithSpecField("claimRef", "foo").
				WithSpecField("storageClassName", "foo").
				WithSpecField("foo", "bar").
				Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(false).Restore,
			expectedErr: false,
			expectedRes: NewTestUnstructured().
				WithName("pv-1").
				WithSpecField("foo", "bar").
				Unstructured,
		},
		{
			name:        "when RestorePVs=true, AWS volume ID should be set correctly",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1"}}}},
			volumeMap:   map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1"}: "volume-1"},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", map[string]interface{}{"volumeID": "volume-1"}).Unstructured,
		},
		{
			name:        "when RestorePVs=true, GCE pdName should be set correctly",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("gcePersistentDisk", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1"}}}},
			volumeMap:   map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1"}: "volume-1"},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("gcePersistentDisk", map[string]interface{}{"pdName": "volume-1"}).Unstructured,
		},
		{
			name:        "when RestorePVs=true, Azure pdName should be set correctly",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("azureDisk", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1"}}}},
			volumeMap:   map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1"}: "volume-1"},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("azureDisk", map[string]interface{}{"diskName": "volume-1"}).Unstructured,
		},
		{
			name:        "when RestorePVs=true, unsupported PV source should not get snapshot restored",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("unsupportedPVSource", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1"}}}},
			volumeMap:   map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1"}: "volume-1"},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("unsupportedPVSource", make(map[string]interface{})).Unstructured,
		},
		{
			name:        "volume type and IOPS are correctly passed to CreateVolume",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:     NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1", Type: "gp", Iops: &iops}}}},
			volumeMap:   map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1", Type: "gp", Iops: &iops}: "volume-1"},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", map[string]interface{}{"volumeID": "volume-1"}).Unstructured,
		},
		{
			name:              "When no SnapshotService, warn if backup has snapshots that will not be restored",
			obj:               NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:           NewDefaultTestRestore().Restore,
			backup:            &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": &api.VolumeBackupInfo{SnapshotID: "snap-1"}}}},
			volumeMap:         map[api.VolumeBackupInfo]string{api.VolumeBackupInfo{SnapshotID: "snap-1"}: "volume-1"},
			noSnapshotService: true,
			expectedErr:       false,
			expectedWarn:      true,
			expectedRes:       NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var snapshotService cloudprovider.SnapshotService
			if !test.noSnapshotService {
				snapshotService = &FakeSnapshotService{RestorableVolumes: test.volumeMap}
			}
			restorer := NewPersistentVolumeRestorer(snapshotService)

			res, warn, err := restorer.Prepare(test.obj, test.restore, test.backup)

			assert.Equal(t, test.expectedWarn, warn != nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}

func TestPVRestorerReady(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "no status returns not ready",
			obj:      NewTestUnstructured().Unstructured,
			expected: false,
		},
		{
			name:     "no status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatus().Unstructured,
			expected: false,
		},
		{
			name:     "empty status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "").Unstructured,
			expected: false,
		},
		{
			name:     "non-Available status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "foo").Unstructured,
			expected: false,
		},
		{
			name:     "Available status.phase returns ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "Available").Unstructured,
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restorer := NewPersistentVolumeRestorer(nil)

			assert.Equal(t, test.expected, restorer.Ready(test.obj))
		})
	}
}
