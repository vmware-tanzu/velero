/*
Copyright 2017 the Heptio Ark contributors.

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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestGCControllerRun(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	tests := []struct {
		name              string
		backups           []*api.Backup
		snapshots         sets.String
		expectedDeletions sets.String
	}{
		{
			name: "no backups results in no deletions",
		},
		{
			name: "expired backup is deleted",
			backups: []*api.Backup{
				arktest.NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(-1*time.Second)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
			expectedDeletions: sets.NewString("backup-1"),
		},
		{
			name: "unexpired backup is not deleted",
			backups: []*api.Backup{
				arktest.NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(1*time.Minute)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
			},
			expectedDeletions: sets.NewString(),
		},
		{
			name: "expired backup is deleted and unexpired backup is not deleted",
			backups: []*api.Backup{
				arktest.NewTestBackup().WithName("backup-1").
					WithExpiration(fakeClock.Now().Add(-1*time.Minute)).
					WithSnapshot("pv-1", "snapshot-1").
					WithSnapshot("pv-2", "snapshot-2").
					Backup,
				arktest.NewTestBackup().WithName("backup-2").
					WithExpiration(fakeClock.Now().Add(1*time.Minute)).
					WithSnapshot("pv-3", "snapshot-3").
					WithSnapshot("pv-4", "snapshot-4").
					Backup,
			},
			expectedDeletions: sets.NewString("backup-1"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			controller := NewGCController(
				nil,
				nil,
				"bucket",
				1*time.Millisecond,
				sharedInformers.Ark().V1().Backups(),
				client.ArkV1(),
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				arktest.NewLogger(),
			).(*gcController)
			controller.clock = fakeClock

			for _, backup := range test.backups {
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(backup)
			}

			expectedDeletions := make([]core.Action, 0, len(test.expectedDeletions))
			for backup := range test.expectedDeletions {
				expectedDeletions = append(expectedDeletions, core.NewDeleteAction(
					api.SchemeGroupVersion.WithResource("backups"),
					api.DefaultNamespace,
					backup,
				))
			}

			controller.run()

			assert.Equal(t, expectedDeletions, client.Actions())
		})
	}
}

func TestGarbageCollectBackup(t *testing.T) {
	tests := []struct {
		name                   string
		backup                 *api.Backup
		snapshots              sets.String
		restores               []*api.Restore
		nilSnapshotService     bool
		expectErr              bool
		expectBackupDirDeleted bool
	}{
		{
			name:               "nil snapshot service when backup has snapshots returns error",
			backup:             arktest.NewTestBackup().WithName("backup-1").WithSnapshot("pv-1", "snap-1").Backup,
			nilSnapshotService: true,
			expectErr:          true,
		},
		{
			name:                   "nil snapshot service when backup doesn't have snapshots correctly garbage-collects",
			backup:                 arktest.NewTestBackup().WithName("backup-1").Backup,
			nilSnapshotService:     true,
			expectBackupDirDeleted: true,
		},
		{
			name: "return error if snapshot deletion fails",
			backup: arktest.NewTestBackup().WithName("backup-1").
				WithSnapshot("pv-1", "snapshot-1").
				WithSnapshot("pv-2", "snapshot-2").
				Backup,
			snapshots:              sets.NewString("snapshot-1"),
			expectBackupDirDeleted: true,
			expectErr:              true,
		},
		{
			name:      "related restores should be deleted",
			backup:    arktest.NewTestBackup().WithName("backup-1").Backup,
			snapshots: sets.NewString(),
			restores: []*api.Restore{
				arktest.NewTestRestore(api.DefaultNamespace, "restore-1", api.RestorePhaseCompleted).WithBackup("backup-1").Restore,
				arktest.NewTestRestore(api.DefaultNamespace, "restore-2", api.RestorePhaseCompleted).WithBackup("backup-2").Restore,
			},
			expectBackupDirDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backupService   = &arktest.BackupService{}
				snapshotService = &arktest.FakeSnapshotService{SnapshotsTaken: test.snapshots}
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				controller      = NewGCController(
					backupService,
					snapshotService,
					"bucket-1",
					1*time.Millisecond,
					sharedInformers.Ark().V1().Backups(),
					client.ArkV1(),
					sharedInformers.Ark().V1().Restores(),
					client.ArkV1(),
					arktest.NewLogger(),
				).(*gcController)
			)

			if test.nilSnapshotService {
				controller.snapshotService = nil
			}

			sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup)
			for _, restore := range test.restores {
				sharedInformers.Ark().V1().Restores().Informer().GetStore().Add(restore)
			}

			if test.expectBackupDirDeleted {
				backupService.On("DeleteBackupDir", controller.bucket, test.backup.Name).Return(nil)
			}

			// METHOD UNDER TEST
			err := controller.garbageCollect(test.backup, controller.logger)

			// VERIFY:

			// error
			assert.Equal(t, test.expectErr, err != nil)

			// remaining snapshots
			if !test.nilSnapshotService {
				backupSnapshots := sets.NewString()
				for _, snapshot := range test.backup.Status.VolumeBackups {
					backupSnapshots.Insert(snapshot.SnapshotID)
				}

				assert.Equal(t, test.snapshots.Difference(backupSnapshots), snapshotService.SnapshotsTaken)
			}

			// restore client deletes
			expectedActions := make([]core.Action, 0)
			for _, restore := range test.restores {
				if restore.Spec.BackupName != test.backup.Name {
					continue
				}

				action := core.NewDeleteAction(
					api.SchemeGroupVersion.WithResource("restores"),
					api.DefaultNamespace,
					restore.Name,
				)
				expectedActions = append(expectedActions, action)
			}
			assert.Equal(t, expectedActions, client.Actions())

			// backup dir deletion
			backupService.AssertExpectations(t)
		})
	}
}

func TestGarbageCollectPicksUpBackupUponExpiration(t *testing.T) {
	var (
		fakeClock       = clock.NewFakeClock(time.Now())
		client          = fake.NewSimpleClientset()
		sharedInformers = informers.NewSharedInformerFactory(client, 0)
		backup          = arktest.NewTestBackup().WithName("backup-1").
				WithExpiration(fakeClock.Now().Add(1*time.Second)).
				WithSnapshot("pv-1", "snapshot-1").
				WithSnapshot("pv-2", "snapshot-2").
				Backup
	)

	controller := NewGCController(
		nil,
		nil,
		"bucket",
		1*time.Millisecond,
		sharedInformers.Ark().V1().Backups(),
		client.ArkV1(),
		sharedInformers.Ark().V1().Restores(),
		client.ArkV1(),
		arktest.NewLogger(),
	).(*gcController)
	controller.clock = fakeClock

	sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(backup)

	// PASS 1
	controller.run()
	assert.Equal(t, 0, len(client.Actions()))

	// PASS 2
	expectedActions := []core.Action{
		core.NewDeleteAction(
			api.SchemeGroupVersion.WithResource("backups"),
			api.DefaultNamespace,
			"backup-1",
		),
	}

	fakeClock.Step(1 * time.Minute)
	controller.run()

	assert.Equal(t, expectedActions, client.Actions())
}

func TestHandleFinalizer(t *testing.T) {
	tests := []struct {
		name                 string
		backup               *api.Backup
		deleteBackupDirError bool
		expectGarbageCollect bool
		expectedPatch        []byte
	}{
		{
			name:   "nil deletionTimestamp exits early",
			backup: arktest.NewTestBackup().Backup,
		},
		{
			name:   "no finalizers exits early",
			backup: arktest.NewTestBackup().WithDeletionTimestamp(time.Now()).Backup,
		},
		{
			name:   "no GCFinalizer exits early",
			backup: arktest.NewTestBackup().WithDeletionTimestamp(time.Now()).WithFinalizers("foo").Backup,
		},
		{
			name:                 "error when calling garbageCollect exits without patch",
			backup:               arktest.NewTestBackup().WithDeletionTimestamp(time.Now()).WithFinalizers(api.GCFinalizer).Backup,
			deleteBackupDirError: true,
		},
		{
			name:                 "normal case - patch includes the appropriate fields",
			backup:               arktest.NewTestBackup().WithDeletionTimestamp(time.Now()).WithFinalizers(api.GCFinalizer, "foo").WithResourceVersion("1").Backup,
			expectGarbageCollect: true,
			expectedPatch:        []byte(`{"metadata":{"finalizers":["foo"],"resourceVersion":"1"}}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backupService   = &arktest.BackupService{}
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				controller      = NewGCController(
					backupService,
					nil,
					"bucket-1",
					1*time.Millisecond,
					sharedInformers.Ark().V1().Backups(),
					client.ArkV1(),
					sharedInformers.Ark().V1().Restores(),
					client.ArkV1(),
					arktest.NewLogger(),
				).(*gcController)
			)

			if test.expectGarbageCollect {
				backupService.On("DeleteBackupDir", controller.bucket, test.backup.Name).Return(nil)
			} else if test.deleteBackupDirError {
				backupService.On("DeleteBackupDir", controller.bucket, test.backup.Name).Return(errors.New("foo"))
			}

			// METHOD UNDER TEST
			controller.handleFinalizer(nil, test.backup)

			// VERIFY
			backupService.AssertExpectations(t)

			actions := client.Actions()

			if test.expectedPatch == nil {
				assert.Equal(t, 0, len(actions))
				return
			}

			require.Equal(t, 1, len(actions))
			patchAction, ok := actions[0].(core.PatchAction)
			require.True(t, ok, "action is not a PatchAction")

			assert.Equal(t, test.expectedPatch, patchAction.GetPatch())
		})
	}
}
