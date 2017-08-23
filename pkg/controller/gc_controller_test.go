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

package controller

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	. "github.com/heptio/ark/pkg/util/test"
)

type gcTest struct {
	name               string
	bucket             string
	backups            map[string][]*api.Backup
	snapshots          sets.String
	nilSnapshotService bool

	expectedBackupsRemaining   map[string]sets.String
	expectedSnapshotsRemaining sets.String
}

func TestGarbageCollect(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	tests := []gcTest{
		gcTest{
			name:   "basic-expired",
			bucket: "bucket-1",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(-1*time.Second)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
				},
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2"),
			expectedBackupsRemaining:   make(map[string]sets.String),
			expectedSnapshotsRemaining: sets.NewString(),
		},
		gcTest{
			name:   "basic-unexpired",
			bucket: "bucket-1",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(1*time.Minute)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
				},
			},
			snapshots: sets.NewString("snapshot-1", "snapshot-2"),
			expectedBackupsRemaining: map[string]sets.String{
				"bucket-1": sets.NewString("backup-1"),
			},
			expectedSnapshotsRemaining: sets.NewString("snapshot-1", "snapshot-2"),
		},
		gcTest{
			name:   "one expired, one unexpired",
			bucket: "bucket-1",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(-1*time.Minute)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
					NewTestBackup().WithName("backup-2").
						WithExpiration(fakeClock.Now().Add(1*time.Minute)).
						WithSnapshot("pv-3", "snapshot-3").
						WithSnapshot("pv-4", "snapshot-4").
						Backup,
				},
			},
			snapshots: sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedBackupsRemaining: map[string]sets.String{
				"bucket-1": sets.NewString("backup-2"),
			},
			expectedSnapshotsRemaining: sets.NewString("snapshot-3", "snapshot-4"),
		},
		gcTest{
			name:   "none expired in target bucket",
			bucket: "bucket-2",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(-1*time.Minute)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
				},
				"bucket-2": []*api.Backup{
					NewTestBackup().WithName("backup-2").
						WithExpiration(fakeClock.Now().Add(1*time.Minute)).
						WithSnapshot("pv-3", "snapshot-3").
						WithSnapshot("pv-4", "snapshot-4").
						Backup,
				},
			},
			snapshots: sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedBackupsRemaining: map[string]sets.String{
				"bucket-1": sets.NewString("backup-1"),
				"bucket-2": sets.NewString("backup-2"),
			},
			expectedSnapshotsRemaining: sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
		},
		gcTest{
			name:   "orphan snapshots",
			bucket: "bucket-1",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(-1*time.Minute)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
				},
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedBackupsRemaining:   make(map[string]sets.String),
			expectedSnapshotsRemaining: sets.NewString("snapshot-3", "snapshot-4"),
		},
		gcTest{
			name:   "no snapshot service only GC's backups without snapshots",
			bucket: "bucket-1",
			backups: map[string][]*api.Backup{
				"bucket-1": []*api.Backup{
					NewTestBackup().WithName("backup-1").
						WithExpiration(fakeClock.Now().Add(-1*time.Second)).
						WithSnapshot("pv-1", "snapshot-1").
						WithSnapshot("pv-2", "snapshot-2").
						Backup,
					NewTestBackup().WithName("backup-2").
						WithExpiration(fakeClock.Now().Add(-1 * time.Second)).
						Backup,
				},
			},
			snapshots:          sets.NewString("snapshot-1", "snapshot-2"),
			nilSnapshotService: true,
			expectedBackupsRemaining: map[string]sets.String{
				"bucket-1": sets.NewString("backup-1"),
			},
		},
	}

	for _, test := range tests {
		var (
			backupService   = &fakeBackupService{}
			snapshotService *FakeSnapshotService
		)

		if !test.nilSnapshotService {
			snapshotService = &FakeSnapshotService{SnapshotsTaken: test.snapshots}
		}

		t.Run(test.name, func(t *testing.T) {
			backupService.backupsByBucket = make(map[string][]*api.Backup)

			for bucket, backups := range test.backups {
				data := make([]*api.Backup, 0, len(backups))
				for _, backup := range backups {
					data = append(data, backup)
				}

				backupService.backupsByBucket[bucket] = data
			}

			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				snapSvc         cloudprovider.SnapshotService
			)

			if snapshotService != nil {
				snapSvc = snapshotService
			}

			controller := NewGCController(
				backupService,
				snapSvc,
				test.bucket,
				1*time.Millisecond,
				sharedInformers.Ark().V1().Backups(),
				client.ArkV1(),
			).(*gcController)
			controller.clock = fakeClock

			controller.cleanBackups()

			// verify every bucket has the backups we expect
			for bucket, backups := range backupService.backupsByBucket {
				// if actual and expected are both empty, no further verification needed
				if len(backups) == 0 && len(test.expectedBackupsRemaining[bucket]) == 0 {
					continue
				}

				// get all the actual backups remaining in this bucket
				backupNames := sets.NewString()
				for _, backup := range backupService.backupsByBucket[bucket] {
					backupNames.Insert(backup.Name)
				}

				assert.Equal(t, test.expectedBackupsRemaining[bucket], backupNames)
			}

			if !test.nilSnapshotService {
				assert.Equal(t, test.expectedSnapshotsRemaining, snapshotService.SnapshotsTaken)
			}
		})
	}
}

func TestGarbageCollectPicksUpBackupUponExpiration(t *testing.T) {
	var (
		backupService   = &fakeBackupService{}
		snapshotService = &FakeSnapshotService{}
		fakeClock       = clock.NewFakeClock(time.Now())
		assert          = assert.New(t)
	)

	scenario := gcTest{
		name:   "basic-expired",
		bucket: "bucket-1",
		backups: map[string][]*api.Backup{
			"bucket-1": []*api.Backup{
				NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(1*time.Second)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
		},
		snapshots: sets.NewString("snapshot-1", "snapshot-2"),
	}

	backupService.backupsByBucket = make(map[string][]*api.Backup)

	for bucket, backups := range scenario.backups {
		data := make([]*api.Backup, 0, len(backups))
		for _, backup := range backups {
			data = append(data, backup)
		}

		backupService.backupsByBucket[bucket] = data
	}

	snapshotService.SnapshotsTaken = scenario.snapshots

	var (
		client          = fake.NewSimpleClientset()
		sharedInformers = informers.NewSharedInformerFactory(client, 0)
	)

	controller := NewGCController(
		backupService,
		snapshotService,
		scenario.bucket,
		1*time.Millisecond,
		sharedInformers.Ark().V1().Backups(),
		client.ArkV1(),
	).(*gcController)
	controller.clock = fakeClock

	// PASS 1
	controller.cleanBackups()

	assert.Equal(scenario.backups, backupService.backupsByBucket, "backups should not be garbage-collected yet.")
	assert.Equal(scenario.snapshots, snapshotService.SnapshotsTaken, "snapshots should not be garbage-collected yet.")

	// PASS 2
	fakeClock.Step(1 * time.Minute)
	controller.cleanBackups()

	assert.Equal(0, len(backupService.backupsByBucket[scenario.bucket]), "backups should have been garbage-collected.")
	assert.Equal(0, len(snapshotService.SnapshotsTaken), "snapshots should have been garbage-collected.")
}

type fakeBackupService struct {
	backupsByBucket map[string][]*api.Backup
	mock.Mock
}

func (s *fakeBackupService) GetAllBackups(bucket string) ([]*api.Backup, error) {
	backups, found := s.backupsByBucket[bucket]
	if !found {
		return nil, errors.New("bucket not found")
	}
	return backups, nil
}

func (bs *fakeBackupService) UploadBackup(bucket, name string, metadata, backup io.ReadSeeker) error {
	args := bs.Called(bucket, name, metadata, backup)
	return args.Error(0)
}

func (s *fakeBackupService) DownloadBackup(bucket, name string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil
}

func (s *fakeBackupService) DeleteBackup(bucket, backupName string) error {
	backups, err := s.GetAllBackups(bucket)
	if err != nil {
		return err
	}

	deleteIdx := -1
	for i, backup := range backups {
		if backup.Name == backupName {
			deleteIdx = i
			break
		}
	}

	if deleteIdx == -1 {
		return errors.New("backup not found")
	}

	s.backupsByBucket[bucket] = append(s.backupsByBucket[bucket][0:deleteIdx], s.backupsByBucket[bucket][deleteIdx+1:]...)

	return nil
}
