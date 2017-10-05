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
	"testing"
	"time"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	. "github.com/heptio/ark/pkg/util/test"
)

type gcTest struct {
	name               string
	backups            []*api.Backup
	snapshots          sets.String
	nilSnapshotService bool

	expectedDeletions          sets.String
	expectedSnapshotsRemaining sets.String
}

func TestGarbageCollect(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	tests := []gcTest{
		gcTest{
			name: "basic-expired",
			backups: []*api.Backup{
				NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(-1*time.Second)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2"),
			expectedDeletions:          sets.NewString("backup-1"),
			expectedSnapshotsRemaining: sets.NewString(),
		},
		gcTest{
			name: "basic-unexpired",
			backups: []*api.Backup{
				NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(1*time.Minute)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2"),
			expectedDeletions:          sets.NewString(),
			expectedSnapshotsRemaining: sets.NewString("snapshot-1", "snapshot-2"),
		},
		gcTest{
			name: "one expired, one unexpired",
			backups: []*api.Backup{
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
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedDeletions:          sets.NewString("backup-1"),
			expectedSnapshotsRemaining: sets.NewString("snapshot-3", "snapshot-4"),
		},
		gcTest{
			name: "none expired in target bucket",
			backups: []*api.Backup{
				NewTestBackup().WithName("backup-2").
					WithExpiration(fakeClock.Now().Add(1*time.Minute)).
					WithSnapshot("pv-3", "snapshot-3").
					WithSnapshot("pv-4", "snapshot-4").
					Backup,
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedDeletions:          sets.NewString(),
			expectedSnapshotsRemaining: sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
		},
		gcTest{
			name: "orphan snapshots",
			backups: []*api.Backup{
				NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(-1*time.Minute)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
			snapshots:                  sets.NewString("snapshot-1", "snapshot-2", "snapshot-3", "snapshot-4"),
			expectedDeletions:          sets.NewString("backup-1"),
			expectedSnapshotsRemaining: sets.NewString("snapshot-3", "snapshot-4"),
		},
		gcTest{
			name: "no snapshot service only GC's backups without snapshots",
			backups: []*api.Backup{
				NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(-1*time.Second)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
				NewTestBackup().WithName("backup-2").
					WithExpiration(fakeClock.Now().Add(-1 * time.Second)).
					Backup,
			},
			snapshots:          sets.NewString("snapshot-1", "snapshot-2"),
			nilSnapshotService: true,
			expectedDeletions:  sets.NewString("backup-2"),
		},
	}

	for _, test := range tests {
		var (
			backupService   = &BackupService{}
			snapshotService *FakeSnapshotService
		)

		if !test.nilSnapshotService {
			snapshotService = &FakeSnapshotService{SnapshotsTaken: test.snapshots}
		}

		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				snapSvc         cloudprovider.SnapshotService
				bucket          = "bucket"
				logger, _       = testlogger.NewNullLogger()
			)

			if snapshotService != nil {
				snapSvc = snapshotService
			}

			controller := NewGCController(
				backupService,
				snapSvc,
				bucket,
				1*time.Millisecond,
				sharedInformers.Ark().V1().Backups(),
				client.ArkV1(),
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				logger,
			).(*gcController)
			controller.clock = fakeClock

			backupService.On("GetAllBackups", bucket).Return(test.backups, nil)
			for _, b := range test.expectedDeletions.List() {
				backupService.On("DeleteBackupDir", bucket, b).Return(nil)
			}

			controller.processBackups()

			if !test.nilSnapshotService {
				assert.Equal(t, test.expectedSnapshotsRemaining, snapshotService.SnapshotsTaken)
			}

			backupService.AssertExpectations(t)
		})
	}
}

func TestGarbageCollectBackup(t *testing.T) {
	tests := []struct {
		name                           string
		backup                         *api.Backup
		deleteBackupFile               bool
		snapshots                      sets.String
		backupFiles                    sets.String
		backupMetadataFiles            sets.String
		restores                       []*api.Restore
		expectedRestoreDeletes         []string
		expectedBackupDelete           string
		expectedSnapshots              sets.String
		expectedObjectStorageDeletions sets.String
	}{
		{
			name: "deleteBackupFile=false, snapshot deletion fails, don't delete kube backup",
			backup: NewTestBackup().WithName("backup-1").
				WithSnapshot("pv-1", "snapshot-1").
				WithSnapshot("pv-2", "snapshot-2").
				Backup,
			deleteBackupFile:               false,
			snapshots:                      sets.NewString("snapshot-1"),
			expectedSnapshots:              sets.NewString(),
			expectedObjectStorageDeletions: sets.NewString(),
		},
		{
			name:             "related restores should be deleted",
			backup:           NewTestBackup().WithName("backup-1").Backup,
			deleteBackupFile: true,
			snapshots:        sets.NewString(),
			restores: []*api.Restore{
				NewTestRestore(api.DefaultNamespace, "restore-1", api.RestorePhaseCompleted).WithBackup("backup-1").Restore,
				NewTestRestore(api.DefaultNamespace, "restore-2", api.RestorePhaseCompleted).WithBackup("backup-2").Restore,
			},
			expectedRestoreDeletes:         []string{"restore-1"},
			expectedBackupDelete:           "backup-1",
			expectedSnapshots:              sets.NewString(),
			expectedObjectStorageDeletions: sets.NewString("backup-1"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backupService   = &BackupService{}
				snapshotService = &FakeSnapshotService{SnapshotsTaken: test.snapshots}
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				bucket          = "bucket-1"
				logger, _       = testlogger.NewNullLogger()
				controller      = NewGCController(
					backupService,
					snapshotService,
					bucket,
					1*time.Millisecond,
					sharedInformers.Ark().V1().Backups(),
					client.ArkV1(),
					sharedInformers.Ark().V1().Restores(),
					client.ArkV1(),
					logger,
				).(*gcController)
			)

			sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup)
			for _, restore := range test.restores {
				sharedInformers.Ark().V1().Restores().Informer().GetStore().Add(restore)
			}

			for _, b := range test.expectedObjectStorageDeletions.List() {
				backupService.On("DeleteBackupDir", bucket, b).Return(nil)
			}

			// METHOD UNDER TEST
			controller.garbageCollectBackup(test.backup, test.deleteBackupFile)

			// VERIFY:

			// remaining snapshots
			assert.Equal(t, test.expectedSnapshots, snapshotService.SnapshotsTaken)

			expectedActions := make([]core.Action, 0)
			// Restore client deletes
			for _, restore := range test.expectedRestoreDeletes {
				action := core.NewDeleteAction(
					api.SchemeGroupVersion.WithResource("restores"),
					api.DefaultNamespace,
					restore,
				)
				expectedActions = append(expectedActions, action)
			}

			// Backup client deletes
			if test.expectedBackupDelete != "" {
				action := core.NewDeleteAction(
					api.SchemeGroupVersion.WithResource("backups"),
					api.DefaultNamespace,
					test.expectedBackupDelete,
				)
				expectedActions = append(expectedActions, action)
			}

			assert.Equal(t, expectedActions, client.Actions())

			backupService.AssertExpectations(t)
		})
	}
}

func TestGarbageCollectPicksUpBackupUponExpiration(t *testing.T) {
	var (
		backupService   = &BackupService{}
		snapshotService = &FakeSnapshotService{}
		fakeClock       = clock.NewFakeClock(time.Now())
		assert          = assert.New(t)
	)

	scenario := gcTest{
		name: "basic-expired",
		backups: []*api.Backup{
			NewTestBackup().WithName("backup-1").
				WithExpiration(fakeClock.Now().Add(1*time.Second)).
				WithSnapshot("pv-1", "snapshot-1").
				WithSnapshot("pv-2", "snapshot-2").
				Backup,
		},
		snapshots: sets.NewString("snapshot-1", "snapshot-2"),
	}

	snapshotService.SnapshotsTaken = scenario.snapshots

	var (
		client          = fake.NewSimpleClientset()
		sharedInformers = informers.NewSharedInformerFactory(client, 0)
		logger, _       = testlogger.NewNullLogger()
	)

	controller := NewGCController(
		backupService,
		snapshotService,
		"bucket",
		1*time.Millisecond,
		sharedInformers.Ark().V1().Backups(),
		client.ArkV1(),
		sharedInformers.Ark().V1().Restores(),
		client.ArkV1(),
		logger,
	).(*gcController)
	controller.clock = fakeClock

	backupService.On("GetAllBackups", "bucket").Return(scenario.backups, nil)

	// PASS 1
	controller.processBackups()

	backupService.AssertExpectations(t)
	assert.Equal(scenario.snapshots, snapshotService.SnapshotsTaken, "snapshots should not be garbage-collected yet.")

	// PASS 2
	fakeClock.Step(1 * time.Minute)
	backupService.On("DeleteBackupDir", "bucket", "backup-1").Return(nil)
	controller.processBackups()

	assert.Equal(0, len(snapshotService.SnapshotsTaken), "snapshots should have been garbage-collected.")

	backupService.AssertExpectations(t)
}
