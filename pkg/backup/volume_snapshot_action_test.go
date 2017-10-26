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

package backup

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestVolumeSnapshotAction(t *testing.T) {
	iops := int64(1000)

	tests := []struct {
		name                   string
		snapshotEnabled        bool
		pv                     string
		ttl                    time.Duration
		expectError            bool
		expectedVolumeID       string
		expectedSnapshotsTaken int
		existingVolumeBackups  map[string]*v1.VolumeBackupInfo
		volumeInfo             map[string]v1.VolumeBackupInfo
	}{
		{
			name:            "snapshot disabled",
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}}`,
			snapshotEnabled: false,
		},
		{
			name:            "can't find volume id - missing spec",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}}`,
			expectError:     true,
		},
		{
			name:            "unsupported PV source type",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"unsupportedPVSource": {}}}`,
			expectError:     false,
		},
		{
			name:            "can't find volume id - aws but no volume id",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"awsElasticBlockStore": {}}}`,
			expectError:     true,
		},
		{
			name:            "can't find volume id - gce but no volume id",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"gcePersistentDisk": {}}}`,
			expectError:     true,
		},
		{
			name:                   "aws - simple volume id",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "gp", SnapshotID: "snap-1", AvailabilityZone: "us-east-1c"},
			},
		},
		{
			name:                   "aws - simple volume id with provisioned IOPS",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "io1", Iops: &iops, SnapshotID: "snap-1", AvailabilityZone: "us-east-1c"},
			},
		},
		{
			name:                   "aws - dynamically provisioned volume id",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-west-2a"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-west-2a/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "gp", SnapshotID: "snap-1", AvailabilityZone: "us-west-2a"},
			},
		},
		{
			name:                   "gce",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "gcp-zone2"}}, "spec": {"gcePersistentDisk": {"pdName": "pd-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "pd-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"pd-abc123": {Type: "gp", SnapshotID: "snap-1", AvailabilityZone: "gcp-zone2"},
			},
		},
		{
			name:                   "azure",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"azureDisk": {"diskName": "foo-disk"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "foo-disk",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"foo-disk": {Type: "gp", SnapshotID: "snap-1"},
			},
		},
		{
			name:                   "preexisting volume backup info in backup status",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"gcePersistentDisk": {"pdName": "pd-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "pd-abc123",
			ttl:                    5 * time.Minute,
			existingVolumeBackups: map[string]*v1.VolumeBackupInfo{
				"anotherpv": {SnapshotID: "anothersnap"},
			},
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"pd-abc123": {Type: "gp", SnapshotID: "snap-1"},
			},
		},
		{
			name:            "create snapshot error",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"gcePersistentDisk": {"pdName": "pd-abc123"}}}`,
			expectError:     true,
		},
		{
			name:                   "PV with label metadata but no failureDomainZone",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/region": "us-east-1"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "gp", SnapshotID: "snap-1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backup := &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1.DefaultNamespace,
					Name:      "mybackup",
				},
				Spec: v1.BackupSpec{
					SnapshotVolumes: &test.snapshotEnabled,
					TTL:             metav1.Duration{Duration: test.ttl},
				},
				Status: v1.BackupStatus{
					VolumeBackups: test.existingVolumeBackups,
				},
			}

			snapshotService := &arktest.FakeSnapshotService{SnapshottableVolumes: test.volumeInfo}

			vsa, _ := NewVolumeSnapshotAction(snapshotService)
			action := vsa.(*volumeSnapshotAction)

			pv, err := getAsMap(test.pv)
			if err != nil {
				t.Fatal(err)
			}

			// method under test
			additionalItems, err := action.Execute(arktest.NewLogger(), &unstructured.Unstructured{Object: pv}, backup)
			assert.Len(t, additionalItems, 0)

			gotErr := err != nil

			if e, a := test.expectError, gotErr; e != a {
				t.Errorf("error: expected %v, got %v", e, a)
			}
			if test.expectError {
				return
			}

			if !test.snapshotEnabled {
				// don't need to check anything else if snapshots are disabled
				return
			}

			expectedVolumeBackups := test.existingVolumeBackups
			if expectedVolumeBackups == nil {
				expectedVolumeBackups = make(map[string]*v1.VolumeBackupInfo)
			}

			// we should have one snapshot taken exactly
			require.Equal(t, test.expectedSnapshotsTaken, snapshotService.SnapshotsTaken.Len())

			if test.expectedSnapshotsTaken > 0 {
				// the snapshotID should be the one in the entry in snapshotService.SnapshottableVolumes
				// for the volume we ran the test for
				snapshotID, _ := snapshotService.SnapshotsTaken.PopAny()

				expectedVolumeBackups["mypv"] = &v1.VolumeBackupInfo{
					SnapshotID:       snapshotID,
					Type:             test.volumeInfo[test.expectedVolumeID].Type,
					Iops:             test.volumeInfo[test.expectedVolumeID].Iops,
					AvailabilityZone: test.volumeInfo[test.expectedVolumeID].AvailabilityZone,
				}

				if e, a := expectedVolumeBackups, backup.Status.VolumeBackups; !reflect.DeepEqual(e, a) {
					t.Errorf("backup.status.VolumeBackups: expected %v, got %v", e, a)
				}
			}
		})
	}
}
