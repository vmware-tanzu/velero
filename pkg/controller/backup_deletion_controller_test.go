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

package controller

import (
	"bytes"
	"fmt"

	"io/ioutil"
	"testing"
	"time"

	"context"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type backupDeletionControllerTestData struct {
	fakeClient        client.Client
	sharedInformers   informers.SharedInformerFactory
	volumeSnapshotter *velerotest.FakeVolumeSnapshotter
	backupStore       *persistencemocks.BackupStore
	controller        *backupDeletionReconciler
	req               ctrl.Request
}

func defaultTestDbr() *velerov1api.DeleteBackupRequest {
	req := pkgbackup.NewDeleteBackupRequest("foo", "uid")
	req.Namespace = velerov1api.DefaultNamespace
	req.Name = "foo-abcde"
	return req
}

func setupBackupDeletionControllerTest(t *testing.T, req *velerov1api.DeleteBackupRequest, objects ...runtime.Object) *backupDeletionControllerTestData {

	var (
		fakeClient        = velerotest.NewFakeControllerRuntimeClient(t, append(objects, req)...)
		volumeSnapshotter = &velerotest.FakeVolumeSnapshotter{SnapshotsTaken: sets.NewString()}
		pluginManager     = &pluginmocks.Manager{}
		backupStore       = &persistencemocks.BackupStore{}
	)

	data := &backupDeletionControllerTestData{
		fakeClient:        fakeClient,
		volumeSnapshotter: volumeSnapshotter,
		backupStore:       backupStore,
		controller: NewBackupDeletionReconciler(
			velerotest.NewLogger(),
			fakeClient,
			NewBackupTracker(),
			nil, // restic repository manager
			metrics.NewServerMetrics(),
			nil, // discovery helper
			func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			NewFakeSingleObjectBackupStoreGetter(backupStore),
		),
		req: ctrl.Request{NamespacedName: types.NamespacedName{Namespace: req.Namespace, Name: req.Name}},
	}

	pluginManager.On("CleanupClients").Return(nil)
	return data
}

func TestBackupDeletionControllerReconcile(t *testing.T) {
	t.Run("failed to get backup store", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()
		location := &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      backup.Spec.StorageLocation,
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket",
					},
				},
			},
		}
		td := setupBackupDeletionControllerTest(t, defaultTestDbr(), location, backup)
		td.controller.backupStoreGetter = &fakeErrorBackupStoreGetter{}
		_, err := td.controller.Reconcile(ctx, td.req)
		assert.NotNil(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "error getting the backup store"))
	})

func TestBackupDeletionControllerProcessRequest(t *testing.T) {
	t.Run("missing spec.backupName", func(t *testing.T) {
		dbr := defaultTestDbr()
		dbr.Spec.BackupName = ""
		td := setupBackupDeletionControllerTest(t, dbr)

		_, err := td.controller.Reconcile(ctx, td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "spec.backupName is required", res.Status.Errors[0])
	})

	t.Run("existing deletion requests for the backup are deleted", func(t *testing.T) {
		input := defaultTestDbr()
		td := setupBackupDeletionControllerTest(t, input)

		// add the backup to the tracker so the execution of reconcile doesn't progress
		// past checking for an in-progress backup. this makes validation easier.

		td.controller.backupTracker.Add(td.req.Namespace, td.req.Spec.BackupName, &pkgbackup.Request{})

		require.NoError(t, td.sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(td.req))

		existing := &velerov1api.DeleteBackupRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: td.req.Namespace,
				Name:      "bar",
				Labels: map[string]string{
					velerov1api.BackupNameLabel: input.Spec.BackupName,
				},
			},
			Spec: velerov1api.DeleteBackupRequestSpec{
				BackupName: input.Spec.BackupName,
			},
		}
		err := td.fakeClient.Create(context.TODO(), existing)
		require.NoError(t, err)
		existing2 :=
			&velerov1api.DeleteBackupRequest{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: td.req.Namespace,
					Name:      "bar-2",
					Labels: map[string]string{
						velerov1api.BackupNameLabel: "some-other-backup",
					},
				},
				Spec: velerov1api.DeleteBackupRequestSpec{
					BackupName: "some-other-backup",
				},
			}
		err = td.fakeClient.Create(context.TODO(), existing2)
		require.NoError(t, err)
		_, err = td.controller.Reconcile(context.TODO(), td.req)
		assert.NoError(t, err)
		// verify "existing" is deleted
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: existing.Namespace,
			Name:      existing.Name,
		}, &velerov1api.DeleteBackupRequest{})

		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		// verify "existing2" remains
		assert.NoError(t, td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: existing2.Namespace,
			Name:      existing2.Name,
		}, &velerov1api.DeleteBackupRequest{}))
	})
	t.Run("deleting an in progress backup isn't allowed", func(t *testing.T) {
		dbr := defaultTestDbr()
		td := setupBackupDeletionControllerTest(t, dbr)

		td.controller.backupTracker.Add(td.req.Namespace, td.req.Spec.BackupName, &pkgbackup.Request{})

		td.controller.backupTracker.Add(td.req.Namespace, dbr.Spec.BackupName)
		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "backup is still in progress", res.Status.Errors[0])
	})

	t.Run("unable to find backup", func(t *testing.T) {

		td := setupBackupDeletionControllerTest(t, defaultTestDbr())

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "backup not found", res.Status.Errors[0])
	})
	t.Run("unable to find backup storage location", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()

		td := setupBackupDeletionControllerTest(t, defaultTestDbr(), backup)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "backup storage location default not found", res.Status.Errors[0])
	})

	t.Run("backup storage location is in read-only mode", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()
		location := builder.ForBackupStorageLocation("velero", "default").AccessMode(velerov1api.BackupStorageLocationAccessModeReadOnly).Result()

		td := setupBackupDeletionControllerTest(t, defaultTestDbr(), location, backup)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "cannot delete backup because backup storage location default is currently in read-only mode", res.Status.Errors[0])
	})
	t.Run("full delete, no errors", func(t *testing.T) {

		input := defaultTestDbr()

		// Clear out resource labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		input.Labels = nil

		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		restore1 := builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Phase(velerov1api.RestorePhaseCompleted).Backup("foo").Result()
		restore2 := builder.ForRestore(velerov1api.DefaultNamespace, "restore-2").Phase(velerov1api.RestorePhaseCompleted).Backup("foo").Result()
		restore3 := builder.ForRestore(velerov1api.DefaultNamespace, "restore-3").Phase(velerov1api.RestorePhaseCompleted).Backup("some-other-backup").Result()

		location := &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      backup.Spec.StorageLocation,
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket",
					},
				},
			},
		}

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		td := setupBackupDeletionControllerTest(t, input, backup, restore1, restore2, restore3, location, snapshotLocation)

		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		snapshots := []*volume.Snapshot{
			{
				Spec: volume.SnapshotSpec{
					Location: "vsl-1",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "snap-1",
				},
			},
		}

		pluginManager := &pluginmocks.Manager{}
		pluginManager.On("GetVolumeSnapshotter", "provider-1").Return(td.volumeSnapshotter, nil)
		pluginManager.On("GetDeleteItemActions").Return(nil, nil)
		pluginManager.On("CleanupClients")
		td.controller.newPluginManager = func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager }

		td.backupStore.On("GetBackupVolumeSnapshots", input.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", input.Spec.BackupName).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)
		td.backupStore.On("DeleteBackup", input.Spec.BackupName).Return(nil)
		td.backupStore.On("DeleteRestore", "restore-1").Return(nil)
		td.backupStore.On("DeleteRestore", "restore-2").Return(nil)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		// the dbr should be deleted
		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)
		if err == nil {
			t.Logf("status of the dbr: %s, errors in dbr: %v", res.Status.Phase, res.Status.Errors)
		}

		// backup CR, restore CR restore-1 and restore-2 should be deleted
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      backup.Name,
		}, &velerov1api.Backup{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-1",
		}, &velerov1api.Restore{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-2",
		}, &velerov1api.Restore{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		// restore-3 should remain
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-3",
		}, &velerov1api.Restore{})
		assert.Nil(t, err)

		td.backupStore.AssertCalled(t, "DeleteBackup", input.Spec.BackupName)
		td.backupStore.AssertCalled(t, "DeleteRestore", "restore-1")
		td.backupStore.AssertCalled(t, "DeleteRestore", "restore-2")

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})
	t.Run("full delete, no errors, with backup name greater than 63 chars", func(t *testing.T) {
		backup := defaultBackup().
			ObjectMeta(
				builder.WithName("the-really-long-backup-name-that-is-much-more-than-63-characters"),
			).
			Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		restore1 := builder.ForRestore(backup.Namespace, "restore-1").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup(backup.Name).
			Result()
		restore2 := builder.ForRestore(backup.Namespace, "restore-2").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup(backup.Name).
			Result()
		restore3 := builder.ForRestore(backup.Namespace, "restore-3").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup("some-other-backup").
			Result()

		dbr := pkgbackup.NewDeleteBackupRequest(backup.Name, "uid")
		dbr.Namespace = velerov1api.DefaultNamespace
		dbr.Name = "foo-abcde"
		// Clear out resource labels to make sure the controller adds them
		dbr.Labels = nil

		location := &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      backup.Spec.StorageLocation,
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket",
					},
				},
			},
		}

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		td := setupBackupDeletionControllerTest(t, dbr, backup, restore1, restore2, restore3, location, snapshotLocation)

		snapshots := []*volume.Snapshot{
			{
				Spec: volume.SnapshotSpec{
					Location: "vsl-1",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "snap-1",
				},
			},
		}

		pluginManager := &pluginmocks.Manager{}
		pluginManager.On("GetVolumeSnapshotter", "provider-1").Return(td.volumeSnapshotter, nil)
		pluginManager.On("GetDeleteItemActions").Return(nil, nil)
		pluginManager.On("CleanupClients")
		td.controller.newPluginManager = func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager }

		td.backupStore.On("GetBackupVolumeSnapshots", dbr.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", dbr.Spec.BackupName).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)
		td.backupStore.On("DeleteBackup", dbr.Spec.BackupName).Return(nil)
		td.backupStore.On("DeleteRestore", "restore-1").Return(nil)
		td.backupStore.On("DeleteRestore", "restore-2").Return(nil)

		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		// the dbr should be deleted
		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)
		if err == nil {
			t.Logf("status of the dbr: %s, errors in dbr: %v", res.Status.Phase, res.Status.Errors)
		}

		// backup CR, restore CR restore-1 and restore-2 should be deleted
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      backup.Name,
		}, &velerov1api.Backup{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-1",
		}, &velerov1api.Restore{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-2",
		}, &velerov1api.Restore{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		// restore-3 should remain
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "restore-3",
		}, &velerov1api.Restore{})
		assert.Nil(t, err)

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})
	t.Run("backup is not downloaded when there are no DeleteItemAction plugins", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		input := defaultTestDbr()
		// Clear out resource labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		input.Labels = nil

		location := &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      backup.Spec.StorageLocation,
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket",
					},
				},
			},
		}

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		td := setupBackupDeletionControllerTest(t, defaultTestDbr(), backup, location, snapshotLocation)
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		snapshots := []*volume.Snapshot{
			{
				Spec: volume.SnapshotSpec{
					Location: "vsl-1",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "snap-1",
				},
			},
		}

		pluginManager := &pluginmocks.Manager{}
		pluginManager.On("GetVolumeSnapshotter", "provider-1").Return(td.volumeSnapshotter, nil)
		pluginManager.On("GetDeleteItemActions").Return([]velero.DeleteItemAction{}, nil)
		pluginManager.On("CleanupClients")
		td.controller.newPluginManager = func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager }

		td.backupStore.On("GetBackupVolumeSnapshots", input.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("DeleteBackup", input.Spec.BackupName).Return(nil)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		td.backupStore.AssertNotCalled(t, "GetBackupContents", mock.Anything)
		td.backupStore.AssertCalled(t, "DeleteBackup", input.Spec.BackupName)

		// the dbr should be deleted
		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)
		if err == nil {
			t.Logf("status of the dbr: %s, errors in dbr: %v", res.Status.Phase, res.Status.Errors)
		}

		// backup CR should be deleted
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      backup.Name,
		}, &velerov1api.Backup{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})
	t.Run("backup is still deleted if downloading tarball fails for DeleteItemAction plugins", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		input := defaultTestDbr()
		// Clear out resource labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		input.Labels = nil

		location := &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      backup.Spec.StorageLocation,
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket",
					},
				},
			},
		}

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		td := setupBackupDeletionControllerTest(t, defaultTestDbr(), backup, location, snapshotLocation)
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		snapshots := []*volume.Snapshot{
			{
				Spec: volume.SnapshotSpec{
					Location: "vsl-1",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "snap-1",
				},
			},
		}

		pluginManager := &pluginmocks.Manager{}
		pluginManager.On("GetVolumeSnapshotter", "provider-1").Return(td.volumeSnapshotter, nil)
		pluginManager.On("GetDeleteItemActions").Return([]velero.DeleteItemAction{new(mocks.DeleteItemAction)}, nil)
		pluginManager.On("CleanupClients")
		td.controller.newPluginManager = func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager }

		td.backupStore.On("GetBackupVolumeSnapshots", input.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", input.Spec.BackupName).Return(nil, fmt.Errorf("error downloading tarball"))
		td.backupStore.On("DeleteBackup", input.Spec.BackupName).Return(nil)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		td.backupStore.AssertCalled(t, "GetBackupContents", input.Spec.BackupName)
		td.backupStore.AssertCalled(t, "DeleteBackup", input.Spec.BackupName)

		// the dbr should be deleted
		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)
		if err == nil {
			t.Logf("status of the dbr: %s, errors in dbr: %v", res.Status.Phase, res.Status.Errors)
		}

		// backup CR should be deleted
		err = td.fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: velerov1api.DefaultNamespace,
			Name:      backup.Name,
		}, &velerov1api.Backup{})
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})
	t.Run("Expired request will be deleted if the status is processed", func(t *testing.T) {
		expired := time.Date(2018, 4, 3, 12, 0, 0, 0, time.UTC)
		input := defaultTestDbr()
		input.CreationTimestamp = metav1.Time{
			Time: expired,
		}
		input.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
		td := setupBackupDeletionControllerTest(t, input)
		td.backupStore.On("DeleteBackup", mock.Anything).Return(nil)
		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		assert.True(t, apierrors.IsNotFound(err), "Expected not found error, but actual value of error: %v", err)
		td.backupStore.AssertNotCalled(t, "DeleteBackup", mock.Anything)

	})

	t.Run("Expired request will not be deleted if the status is not processed", func(t *testing.T) {
		expired := time.Date(2018, 4, 3, 12, 0, 0, 0, time.UTC)
		input := defaultTestDbr()
		input.CreationTimestamp = metav1.Time{
			Time: expired,
		}
		input.Status.Phase = velerov1api.DeleteBackupRequestPhaseNew
		td := setupBackupDeletionControllerTest(t, input)
		td.backupStore.On("DeleteBackup", mock.Anything).Return(nil)

		_, err := td.controller.Reconcile(context.TODO(), td.req)
		require.NoError(t, err)

		res := &velerov1api.DeleteBackupRequest{}
		err = td.fakeClient.Get(ctx, td.req.NamespacedName, res)
		require.NoError(t, err)
		assert.Equal(t, "Processed", string(res.Status.Phase))
		assert.Equal(t, 1, len(res.Status.Errors))
		assert.Equal(t, "backup not found", res.Status.Errors[0])

	})
}
