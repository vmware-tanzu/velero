/*
Copyright 2020 the Velero contributors.

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
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	snapshotv1beta1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	snapshotv1beta1informers "github.com/kubernetes-csi/external-snapshotter/client/v4/informers/externalversions"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

func TestBackupDeletionControllerProcessQueueItem(t *testing.T) {
	client := fake.NewSimpleClientset()
	sharedInformers := informers.NewSharedInformerFactory(client, 0)

	controller := NewBackupDeletionController(
		velerotest.NewLogger(),
		sharedInformers.Velero().V1().DeleteBackupRequests(),
		client.VeleroV1(), // deleteBackupRequestClient
		client.VeleroV1(), // backupClient
		sharedInformers.Velero().V1().Restores().Lister(),
		client.VeleroV1(), // restoreClient
		NewBackupTracker(),
		nil, // restic repository manager
		sharedInformers.Velero().V1().PodVolumeBackups().Lister(),
		nil,
		sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
		nil, // csiSnapshotLister
		nil, // csiSnapshotContentLister
		nil, // csiSnapshotClient
		nil, // new plugin manager func
		metrics.NewServerMetrics(),
		nil, // discovery helper
	).(*backupDeletionController)

	// Error splitting key
	err := controller.processQueueItem("foo/bar/baz")
	assert.Error(t, err)

	// Can't find DeleteBackupRequest
	err = controller.processQueueItem("foo/bar")
	assert.NoError(t, err)

	// Already processed
	req := pkgbackup.NewDeleteBackupRequest("foo", "uid")
	req.Namespace = "foo"
	req.Name = "foo-abcde"
	req.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed

	err = controller.processQueueItem("foo/bar")
	assert.NoError(t, err)

	// Invoke processRequestFunc
	for _, phase := range []velerov1api.DeleteBackupRequestPhase{"", velerov1api.DeleteBackupRequestPhaseNew, velerov1api.DeleteBackupRequestPhaseInProgress} {
		t.Run(fmt.Sprintf("phase=%s", phase), func(t *testing.T) {
			req.Status.Phase = phase
			sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(req)

			var errorToReturn error
			var actual *velerov1api.DeleteBackupRequest
			var called bool
			controller.processRequestFunc = func(r *velerov1api.DeleteBackupRequest) error {
				called = true
				actual = r
				return errorToReturn
			}

			// No error
			err = controller.processQueueItem("foo/foo-abcde")
			require.True(t, called, "processRequestFunc wasn't called")
			assert.Equal(t, err, errorToReturn)
			assert.Equal(t, req, actual)

			// Error
			errorToReturn = errors.New("bar")
			err = controller.processQueueItem("foo/foo-abcde")
			require.True(t, called, "processRequestFunc wasn't called")
			assert.Equal(t, err, errorToReturn)
		})
	}
}

type backupDeletionControllerTestData struct {
	client            *fake.Clientset
	fakeClient        client.Client
	sharedInformers   informers.SharedInformerFactory
	volumeSnapshotter *velerotest.FakeVolumeSnapshotter
	backupStore       *persistencemocks.BackupStore
	controller        *backupDeletionController
	req               *velerov1api.DeleteBackupRequest
}

func setupBackupDeletionControllerTest(t *testing.T, objects ...runtime.Object) *backupDeletionControllerTestData {
	req := pkgbackup.NewDeleteBackupRequest("foo", "uid")
	req.Namespace = "velero"
	req.Name = "foo-abcde"

	var (
		client            = fake.NewSimpleClientset(append(objects, req)...)
		fakeClient        = newFakeClient(t, objects...)
		sharedInformers   = informers.NewSharedInformerFactory(client, 0)
		volumeSnapshotter = &velerotest.FakeVolumeSnapshotter{SnapshotsTaken: sets.NewString()}
		pluginManager     = &pluginmocks.Manager{}
		backupStore       = &persistencemocks.BackupStore{}
	)

	data := &backupDeletionControllerTestData{
		client:            client,
		fakeClient:        fakeClient,
		sharedInformers:   sharedInformers,
		volumeSnapshotter: volumeSnapshotter,
		backupStore:       backupStore,
		controller: NewBackupDeletionController(
			velerotest.NewLogger(),
			sharedInformers.Velero().V1().DeleteBackupRequests(),
			client.VeleroV1(), // deleteBackupRequestClient
			client.VeleroV1(), // backupClient
			sharedInformers.Velero().V1().Restores().Lister(),
			client.VeleroV1(), // restoreClient
			NewBackupTracker(),
			nil, // restic repository manager
			sharedInformers.Velero().V1().PodVolumeBackups().Lister(),
			fakeClient,
			sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
			nil, // csiSnapshotLister
			nil, // csiSnapshotContentLister
			nil, // csiSnapshotClient
			func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			metrics.NewServerMetrics(),
			nil, // discovery helper
		).(*backupDeletionController),

		req: req,
	}

	data.controller.newBackupStore = func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
		return backupStore, nil
	}

	pluginManager.On("CleanupClients").Return(nil)

	return data
}

func TestBackupDeletionControllerProcessRequest(t *testing.T) {
	t.Run("missing spec.backupName", func(t *testing.T) {
		td := setupBackupDeletionControllerTest(t)
		td.req.Spec.BackupName = ""

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"errors":["spec.backupName is required"],"phase":"Processed"}}`),
			),
		}

		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("existing deletion requests for the backup are deleted", func(t *testing.T) {
		td := setupBackupDeletionControllerTest(t)

		// add the backup to the tracker so the execution of processRequest doesn't progress
		// past checking for an in-progress backup. this makes validation easier.
		td.controller.backupTracker.Add(td.req.Namespace, td.req.Spec.BackupName)

		require.NoError(t, td.sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(td.req))

		existing := &velerov1api.DeleteBackupRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: td.req.Namespace,
				Name:      "bar",
				Labels: map[string]string{
					velerov1api.BackupNameLabel: td.req.Spec.BackupName,
				},
			},
			Spec: velerov1api.DeleteBackupRequestSpec{
				BackupName: td.req.Spec.BackupName,
			},
		}
		require.NoError(t, td.sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(existing))
		_, err := td.client.VeleroV1().DeleteBackupRequests(td.req.Namespace).Create(context.TODO(), existing, metav1.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, td.sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(
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
			},
		))

		assert.NoError(t, td.controller.processRequest(td.req))

		expectedDeleteAction := core.NewDeleteAction(
			velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
			td.req.Namespace,
			"bar",
		)

		// first action is the Create of an existing DBR for the backup as part of test data setup
		// second action is the Delete of the existing DBR, which we're validating
		// third action is the Patch of the DBR to set it to processed with an error
		require.Len(t, td.client.Actions(), 3)
		assert.Equal(t, expectedDeleteAction, td.client.Actions()[1])
	})

	t.Run("deleting an in progress backup isn't allowed", func(t *testing.T) {
		td := setupBackupDeletionControllerTest(t)

		td.controller.backupTracker.Add(td.req.Namespace, td.req.Spec.BackupName)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"errors":["backup is still in progress"],"phase":"Processed"}}`),
			),
		}

		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("patching to InProgress fails", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()
		location := builder.ForBackupStorageLocation("velero", "default").Result()

		td := setupBackupDeletionControllerTest(t, location, backup)

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("bad")
		})

		err := td.controller.processRequest(td.req)
		assert.EqualError(t, err, "error patching DeleteBackupRequest: bad")

		expectedActions := []core.Action{
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				backup.Namespace,
				backup.Name,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"InProgress"}}`),
			),
		}
		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("patching backup to Deleting fails", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()
		location := builder.ForBackupStorageLocation("velero", "default").Result()

		td := setupBackupDeletionControllerTest(t, location, backup)

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, td.req, nil
		})
		td.client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("bad")
		})

		err := td.controller.processRequest(td.req)
		assert.EqualError(t, err, "error patching Backup: bad")

		expectedActions := []core.Action{
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				backup.Namespace,
				backup.Name,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"InProgress"}}`),
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				backup.Namespace,
				backup.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Deleting"}}`),
			),
		}
		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("unable to find backup", func(t *testing.T) {
		td := setupBackupDeletionControllerTest(t)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"errors":["backup not found"],"phase":"Processed"}}`),
			),
		}

		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("unable to find backup storage location", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()

		td := setupBackupDeletionControllerTest(t, backup)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"errors":["backup storage location default not found"],"phase":"Processed"}}`),
			),
		}

		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("backup storage location is in read-only mode", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").StorageLocation("default").Result()
		location := builder.ForBackupStorageLocation("velero", "default").AccessMode(velerov1api.BackupStorageLocationAccessModeReadOnly).Result()

		td := setupBackupDeletionControllerTest(t, location, backup)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"errors":["cannot delete backup because backup storage location default is currently in read-only mode"],"phase":"Processed"}}`),
			),
		}

		assert.Equal(t, expectedActions, td.client.Actions())
	})

	t.Run("full delete, no errors", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		restore1 := builder.ForRestore("velero", "restore-1").Phase(velerov1api.RestorePhaseCompleted).Backup("foo").Result()
		restore2 := builder.ForRestore("velero", "restore-2").Phase(velerov1api.RestorePhaseCompleted).Backup("foo").Result()
		restore3 := builder.ForRestore("velero", "restore-3").Phase(velerov1api.RestorePhaseCompleted).Backup("some-other-backup").Result()

		td := setupBackupDeletionControllerTest(t, backup, restore1, restore2, restore3)

		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore1)
		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore2)
		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore3)

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

		require.NoError(t, td.fakeClient.Create(context.Background(), location))

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		require.NoError(t, td.sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(snapshotLocation))

		// Clear out req labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		td.req.Labels = nil

		td.client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, td.req, nil
		})

		td.client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})

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

		td.backupStore.On("GetBackupVolumeSnapshots", td.req.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", td.req.Spec.BackupName).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)
		td.backupStore.On("DeleteBackup", td.req.Spec.BackupName).Return(nil)
		td.backupStore.On("DeleteRestore", "restore-1").Return(nil)
		td.backupStore.On("DeleteRestore", "restore-2").Return(nil)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-name":"foo"}},"status":{"phase":"InProgress"}}`),
			),
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-uid":"uid"}}}`),
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Deleting"}}`),
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("restores"),
				td.req.Namespace,
				"restore-1",
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("restores"),
				td.req.Namespace,
				"restore-2",
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Processed"}}`),
			),
			core.NewDeleteCollectionAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				pkgbackup.NewDeleteBackupRequestListOptions(td.req.Spec.BackupName, "uid"),
			),
		}

		velerotest.CompareActions(t, expectedActions, td.client.Actions())

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

		restore1 := builder.ForRestore("velero", "restore-1").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup("the-really-long-backup-name-that-is-much-more-than-63-characters").
			Result()
		restore2 := builder.ForRestore("velero", "restore-2").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup("the-really-long-backup-name-that-is-much-more-than-63-characters").
			Result()
		restore3 := builder.ForRestore("velero", "restore-3").
			Phase(velerov1api.RestorePhaseCompleted).
			Backup("some-other-backup").
			Result()

		td := setupBackupDeletionControllerTest(t, backup, restore1, restore2, restore3)
		td.req = pkgbackup.NewDeleteBackupRequest(backup.Name, string(backup.UID))
		td.req.Namespace = "velero"
		td.req.Name = "foo-abcde"
		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore1)
		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore2)
		td.sharedInformers.Velero().V1().Restores().Informer().GetStore().Add(restore3)

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
		require.NoError(t, td.fakeClient.Create(context.Background(), location))

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		require.NoError(t, td.sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(snapshotLocation))

		// Clear out req labels to make sure the controller adds them
		td.req.Labels = make(map[string]string)

		td.client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, td.req, nil
		})

		td.client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})

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

		td.backupStore.On("GetBackupVolumeSnapshots", td.req.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", td.req.Spec.BackupName).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)
		td.backupStore.On("DeleteBackup", td.req.Spec.BackupName).Return(nil)
		td.backupStore.On("DeleteRestore", "restore-1").Return(nil)
		td.backupStore.On("DeleteRestore", "restore-2").Return(nil)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-name":"the-really-long-backup-name-that-is-much-more-than-63-cha6ca4bc"}},"status":{"phase":"InProgress"}}`),
			),
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-uid":"uid"}}}`),
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Deleting"}}`),
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("restores"),
				td.req.Namespace,
				"restore-1",
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("restores"),
				td.req.Namespace,
				"restore-2",
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Processed"}}`),
			),
			core.NewDeleteCollectionAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				pkgbackup.NewDeleteBackupRequestListOptions(td.req.Spec.BackupName, "uid"),
			),
		}

		velerotest.CompareActions(t, expectedActions, td.client.Actions())

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})

	t.Run("backup is not downloaded when there are no DeleteItemAction plugins", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		td := setupBackupDeletionControllerTest(t, backup)

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

		require.NoError(t, td.fakeClient.Create(context.Background(), location))

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		require.NoError(t, td.sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(snapshotLocation))

		// Clear out req labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		td.req.Labels = nil

		td.client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, td.req, nil
		})

		td.client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})

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

		td.backupStore.On("GetBackupVolumeSnapshots", td.req.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("DeleteBackup", td.req.Spec.BackupName).Return(nil)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		td.backupStore.AssertNotCalled(t, "GetBackupContents", td.req.Spec.BackupName)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-name":"foo"}},"status":{"phase":"InProgress"}}`),
			),
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-uid":"uid"}}}`),
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Deleting"}}`),
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Processed"}}`),
			),
			core.NewDeleteCollectionAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				pkgbackup.NewDeleteBackupRequestListOptions(td.req.Spec.BackupName, "uid"),
			),
		}

		velerotest.CompareActions(t, expectedActions, td.client.Actions())

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})

	t.Run("backup is still deleted if downloading tarball fails for DeleteItemAction plugins", func(t *testing.T) {
		backup := builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result()
		backup.UID = "uid"
		backup.Spec.StorageLocation = "primary"

		td := setupBackupDeletionControllerTest(t, backup)

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

		require.NoError(t, td.fakeClient.Create(context.Background(), location))

		snapshotLocation := &velerov1api.VolumeSnapshotLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: backup.Namespace,
				Name:      "vsl-1",
			},
			Spec: velerov1api.VolumeSnapshotLocationSpec{
				Provider: "provider-1",
			},
		}
		require.NoError(t, td.sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(snapshotLocation))

		// Clear out req labels to make sure the controller adds them and does not
		// panic when encountering a nil Labels map
		// (https://github.com/vmware-tanzu/velero/issues/1546)
		td.req.Labels = nil

		td.client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})
		td.volumeSnapshotter.SnapshotsTaken.Insert("snap-1")

		td.client.PrependReactor("patch", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
			return true, td.req, nil
		})

		td.client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
			return true, backup, nil
		})

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

		td.backupStore.On("GetBackupVolumeSnapshots", td.req.Spec.BackupName).Return(snapshots, nil)
		td.backupStore.On("GetBackupContents", td.req.Spec.BackupName).Return(nil, fmt.Errorf("error downloading tarball"))
		td.backupStore.On("DeleteBackup", td.req.Spec.BackupName).Return(nil)

		err := td.controller.processRequest(td.req)
		require.NoError(t, err)

		expectedActions := []core.Action{
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-name":"foo"}},"status":{"phase":"InProgress"}}`),
			),
			core.NewGetAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"velero.io/backup-uid":"uid"}}}`),
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Deleting"}}`),
			),
			core.NewDeleteAction(
				velerov1api.SchemeGroupVersion.WithResource("backups"),
				td.req.Namespace,
				td.req.Spec.BackupName,
			),
			core.NewPatchAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				td.req.Name,
				types.MergePatchType,
				[]byte(`{"status":{"phase":"Processed"}}`),
			),
			core.NewDeleteCollectionAction(
				velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"),
				td.req.Namespace,
				pkgbackup.NewDeleteBackupRequestListOptions(td.req.Spec.BackupName, "uid"),
			),
		}

		velerotest.CompareActions(t, expectedActions, td.client.Actions())

		// Make sure snapshot was deleted
		assert.Equal(t, 0, td.volumeSnapshotter.SnapshotsTaken.Len())
	})
}

func TestBackupDeletionControllerDeleteExpiredRequests(t *testing.T) {

	now := time.Date(2018, 4, 4, 12, 0, 0, 0, time.UTC)
	unexpired1 := time.Date(2018, 4, 4, 11, 0, 0, 0, time.UTC)
	unexpired2 := time.Date(2018, 4, 3, 12, 0, 1, 0, time.UTC)
	expired1 := time.Date(2018, 4, 3, 12, 0, 0, 0, time.UTC)
	expired2 := time.Date(2018, 4, 3, 2, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		requests          []*velerov1api.DeleteBackupRequest
		expectedDeletions []string
	}{
		{
			name: "no requests",
		},
		{
			name: "older than max age, phase = '', don't delete",
			requests: []*velerov1api.DeleteBackupRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "name",
						CreationTimestamp: metav1.Time{Time: expired1},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: "",
					},
				},
			},
		},
		{
			name: "older than max age, phase = New, don't delete",
			requests: []*velerov1api.DeleteBackupRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "name",
						CreationTimestamp: metav1.Time{Time: expired1},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseNew,
					},
				},
			},
		},
		{
			name: "older than max age, phase = InProcess, don't delete",
			requests: []*velerov1api.DeleteBackupRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "name",
						CreationTimestamp: metav1.Time{Time: expired1},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseInProgress,
					},
				},
			},
		},
		{
			name: "some expired, some not",
			requests: []*velerov1api.DeleteBackupRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "unexpired-1",
						CreationTimestamp: metav1.Time{Time: unexpired1},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseProcessed,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "expired-1",
						CreationTimestamp: metav1.Time{Time: expired1},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseProcessed,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "unexpired-2",
						CreationTimestamp: metav1.Time{Time: unexpired2},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseProcessed,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "ns",
						Name:              "expired-2",
						CreationTimestamp: metav1.Time{Time: expired2},
					},
					Status: velerov1api.DeleteBackupRequestStatus{
						Phase: velerov1api.DeleteBackupRequestPhaseProcessed,
					},
				},
			},
			expectedDeletions: []string{"expired-1", "expired-2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			fakeClient := newFakeClient(t)
			sharedInformers := informers.NewSharedInformerFactory(client, 0)

			controller := NewBackupDeletionController(
				velerotest.NewLogger(),
				sharedInformers.Velero().V1().DeleteBackupRequests(),
				client.VeleroV1(), // deleteBackupRequestClient
				client.VeleroV1(), // backupClient
				sharedInformers.Velero().V1().Restores().Lister(),
				client.VeleroV1(), // restoreClient
				NewBackupTracker(),
				nil,
				sharedInformers.Velero().V1().PodVolumeBackups().Lister(),
				fakeClient,
				sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				nil, // csiSnapshotLister
				nil, // csiSnapshotContentLister
				nil, // csiSnapshotClient
				nil, // new plugin manager func
				metrics.NewServerMetrics(),
				nil, // discovery helper,
			).(*backupDeletionController)

			fakeClock := &clock.FakeClock{}
			fakeClock.SetTime(now)
			controller.clock = fakeClock

			for i := range test.requests {
				sharedInformers.Velero().V1().DeleteBackupRequests().Informer().GetStore().Add(test.requests[i])
			}

			controller.deleteExpiredRequests()

			expectedActions := []core.Action{}
			for _, name := range test.expectedDeletions {
				expectedActions = append(expectedActions, core.NewDeleteAction(velerov1api.SchemeGroupVersion.WithResource("deletebackuprequests"), "ns", name))
			}

			velerotest.CompareActions(t, expectedActions, client.Actions())
		})
	}
}

func TestSetVolumeSnapshotContentDeletionPolicy(t *testing.T) {
	testCases := []struct {
		name         string
		inputVSCName string
		objs         []runtime.Object
		expectError  bool
	}{
		{
			name:         "should update DeletionPolicy of a VSC from retain to delete",
			inputVSCName: "retainVSC",
			objs: []runtime.Object{
				&snapshotv1beta1api.VolumeSnapshotContent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "retainVSC",
					},
					Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
						DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
					},
				},
			},
			expectError: false,
		},
		{
			name:         "should be a no-op updating if DeletionPolicy of a VSC is already Delete",
			inputVSCName: "deleteVSC",
			objs: []runtime.Object{
				&snapshotv1beta1api.VolumeSnapshotContent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "deleteVSC",
					},
					Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
						DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentDelete,
					},
				},
			},
			expectError: false,
		},
		{
			name:         "should update DeletionPolicy of a VSC with no DeletionPolicy",
			inputVSCName: "nothingVSC",
			objs: []runtime.Object{
				&snapshotv1beta1api.VolumeSnapshotContent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nothingVSC",
					},
					Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{},
				},
			},
			expectError: false,
		},
		{
			name:         "should return not found error if supplied VSC does not exist",
			inputVSCName: "does-not-exist",
			objs:         []runtime.Object{},
			expectError:  true,
		},
	}

	log := velerotest.NewLogger().WithFields(
		logrus.Fields{
			"unit-test": "unit-test",
		},
	)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := snapshotFake.NewSimpleClientset(tc.objs...)
			err := setVolumeSnapshotContentDeletionPolicy(tc.inputVSCName, fakeClient.SnapshotV1beta1(), log)
			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				actual, err := fakeClient.SnapshotV1beta1().VolumeSnapshotContents().Get(context.TODO(), tc.inputVSCName, metav1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, snapshotv1beta1api.VolumeSnapshotContentDelete, actual.Spec.DeletionPolicy)
			}
		})
	}
}

func TestDeleteCSIVolumeSnapshots(t *testing.T) {
	//Backup1
	ns1VS1VSCName := "ns1vs1vsc"
	ns1VS1VSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns1VS1VSCName,
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
		},
	}
	ns1VS1 := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vs1",
			Namespace: "ns1",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup1",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &ns1VS1VSCName,
		},
	}

	ns1VS2VSCName := "ns1vs2vsc"
	ns1VS2VSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns1VS2VSCName,
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
		},
	}
	ns1VS2 := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vs2",
			Namespace: "ns1",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup1",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &ns1VS2VSCName,
		},
	}

	ns2VS1VSCName := "ns2vs1vsc"
	ns2VS1VSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns2VS1VSCName,
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
		},
	}
	ns2VS1 := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vs1",
			Namespace: "ns2",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup1",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &ns2VS1VSCName,
		},
	}

	ns2VS2VSCName := "ns2vs2vsc"
	ns2VS2VSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns2VS2VSCName,
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
		},
	}
	ns2VS2 := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vs2",
			Namespace: "ns2",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup1",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &ns2VS2VSCName,
		},
	}

	// Backup2
	ns1NilStatusVS := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns1NilStatusVS",
			Namespace: "ns2",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup2",
			},
		},
		Status: nil,
	}

	// Backup3
	ns1NilBoundVSCVS := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns1NilBoundVSCVS",
			Namespace: "ns2",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup3",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: nil,
		},
	}

	// Backup4
	notFound := "not-found"
	ns1NonExistentVSCVS := snapshotv1beta1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns1NonExistentVSCVS",
			Namespace: "ns2",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup3",
			},
		},
		Status: &snapshotv1beta1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &notFound,
		},
	}

	testCases := []struct {
		name       string
		backupName string
		objs       []runtime.Object
	}{
		{
			name:       "should delete volumesnapshots bound to existing volumesnapshotcontent",
			backupName: "backup1",
			objs:       []runtime.Object{&ns1VS1VSC, &ns1VS1, &ns1VS2VSC, &ns1VS2, &ns2VS1VSC, &ns2VS1, &ns2VS2VSC, &ns2VS2},
		},
		{
			name:       "should delete volumesnapshots with nil status",
			backupName: "backup2",
			objs:       []runtime.Object{&ns1NilStatusVS},
		},
		{
			name:       "should delete volumesnapshots with nil BoundVolumeSnapshotContentName",
			backupName: "backup3",
			objs:       []runtime.Object{&ns1NilBoundVSCVS},
		},
		{
			name:       "should delete volumesnapshots bound to non-existent volumesnapshotcontents",
			backupName: "backup4",
			objs:       []runtime.Object{&ns1NonExistentVSCVS},
		},
		{
			name:       "should be a no-op when there are no volumesnapshots to delete",
			backupName: "backup-no-vs",
			objs:       []runtime.Object{},
		},
	}

	log := velerotest.NewLogger().WithFields(
		logrus.Fields{
			"unit-test": "unit-test",
		},
	)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := snapshotFake.NewSimpleClientset(tc.objs...)
			fakeSharedInformer := snapshotv1beta1informers.NewSharedInformerFactoryWithOptions(fakeClient, 0)
			for _, o := range tc.objs {
				fakeSharedInformer.Snapshot().V1beta1().VolumeSnapshots().Informer().GetStore().Add(o)
			}
			errs := deleteCSIVolumeSnapshots(tc.backupName, fakeSharedInformer.Snapshot().V1beta1().VolumeSnapshots().Lister(), fakeClient.SnapshotV1beta1(), log)
			assert.Empty(t, errs)
		})
	}
}

func TestDeleteCSIVolumeSnapshotContents(t *testing.T) {
	retainVSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "retainVSC",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup1",
			},
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentRetain,
		},
	}
	deleteVSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deleteVSC",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup2",
			},
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1beta1api.VolumeSnapshotContentDelete,
		},
	}

	nothingVSC := snapshotv1beta1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nothingVSC",
			Labels: map[string]string{
				velerov1api.BackupNameLabel: "backup3",
			},
		},
		Spec: snapshotv1beta1api.VolumeSnapshotContentSpec{},
	}

	testCases := []struct {
		name       string
		backupName string
		objs       []runtime.Object
	}{
		{
			name:       "should delete volumesnapshotcontent with DeletionPolicy Retain",
			backupName: "backup1",
			objs:       []runtime.Object{&retainVSC},
		},
		{
			name:       "should delete volumesnapshotcontent with DeletionPolicy Delete",
			backupName: "backup3",
			objs:       []runtime.Object{&deleteVSC},
		},
		{
			name:       "should delete volumesnapshotcontent with No DeletionPolicy",
			backupName: "backup3",
			objs:       []runtime.Object{&nothingVSC},
		},
		{
			name:       "should return no error when backup has no volumesnapshotconents",
			backupName: "backup-with-no-vsc",
			objs:       []runtime.Object{},
		},
	}
	log := velerotest.NewLogger().WithFields(
		logrus.Fields{
			"unit-test": "unit-test",
		},
	)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := snapshotFake.NewSimpleClientset(tc.objs...)
			fakeSharedInformer := snapshotv1beta1informers.NewSharedInformerFactoryWithOptions(fakeClient, 0)
			for _, o := range tc.objs {
				fakeSharedInformer.Snapshot().V1beta1().VolumeSnapshotContents().Informer().GetStore().Add(o)
			}

			errs := deleteCSIVolumeSnapshotContents(tc.backupName, fakeSharedInformer.Snapshot().V1beta1().VolumeSnapshotContents().Lister(), fakeClient.SnapshotV1beta1(), log)
			assert.Empty(t, errs)
		})
	}
}
