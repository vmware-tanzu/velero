/*
Copyright 2017 the Velero contributors.

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
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"

	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

type downloadRequestTestHarness struct {
	client          *fake.Clientset
	informerFactory informers.SharedInformerFactory
	pluginManager   *pluginmocks.Manager
	backupStore     *persistencemocks.BackupStore

	controller *downloadRequestController
}

func newDownloadRequestTestHarness(t *testing.T, fakeClient client.Client) *downloadRequestTestHarness {
	var (
		client          = fake.NewSimpleClientset()
		informerFactory = informers.NewSharedInformerFactory(client, 0)
		pluginManager   = new(pluginmocks.Manager)
		backupStore     = new(persistencemocks.BackupStore)
		controller      = NewDownloadRequestController(
			client.VeleroV1(),
			informerFactory.Velero().V1().DownloadRequests(),
			informerFactory.Velero().V1().Restores().Lister(),
			fakeClient,
			informerFactory.Velero().V1().Backups().Lister(),
			func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			velerotest.NewLogger(),
		).(*downloadRequestController)
	)

	clockTime, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)
	controller.clock = clock.NewFakeClock(clockTime)

	controller.newBackupStore = func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
		return backupStore, nil
	}

	pluginManager.On("CleanupClients").Return()

	return &downloadRequestTestHarness{
		client:          client,
		informerFactory: informerFactory,
		pluginManager:   pluginManager,
		backupStore:     backupStore,
		controller:      controller,
	}
}

func newDownloadRequest(phase velerov1api.DownloadRequestPhase, targetKind velerov1api.DownloadTargetKind, targetName string) *velerov1api.DownloadRequest {
	return &velerov1api.DownloadRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-download-request",
			Namespace: velerov1api.DefaultNamespace,
		},
		Spec: velerov1api.DownloadRequestSpec{
			Target: velerov1api.DownloadTarget{
				Kind: targetKind,
				Name: targetName,
			},
		},
		Status: velerov1api.DownloadRequestStatus{
			Phase: phase,
		},
	}
}

func newBackupLocation(name, provider, bucket string) *velerov1api.BackupStorageLocation {
	return &velerov1api.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: velerov1api.DefaultNamespace,
		},
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: provider,
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: bucket,
				},
			},
		},
	}
}

func TestProcessDownloadRequest(t *testing.T) {

	defaultBackup := func() *velerov1api.Backup {
		return builder.ForBackup(velerov1api.DefaultNamespace, "a-backup").StorageLocation("a-location").Result()
	}

	tests := []struct {
		name            string
		key             string
		downloadRequest *velerov1api.DownloadRequest
		backup          *velerov1api.Backup
		restore         *velerov1api.Restore
		backupLocation  *velerov1api.BackupStorageLocation
		expired         bool
		expectedErr     string
		expectGetsURL   bool
	}{
		{
			name: "empty key returns without error",
			key:  "",
		},
		{
			name: "bad key format returns without error",
			key:  "a/b/c",
		},
		{
			name: "no download request for key returns without error",
			key:  "nonexistent/key",
		},
		{
			name:            "backup contents request for nonexistent backup returns an error",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindBackupContents, "a-backup"),
			backup:          builder.ForBackup(velerov1api.DefaultNamespace, "non-matching-backup").StorageLocation("a-location").Result(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedErr:     "backup.velero.io \"a-backup\" not found",
		},
		{
			name:            "restore log request for nonexistent restore returns an error",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:         builder.ForRestore(velerov1api.DefaultNamespace, "non-matching-restore").Phase(velerov1api.RestorePhaseCompleted).Backup("a-backup").Result(),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedErr:     "error getting Restore: restore.velero.io \"a-backup-20170912150214\" not found",
		},
		{
			name:            "backup contents request for backup with nonexistent location returns an error",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindBackupContents, "a-backup"),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("non-matching-location", "a-provider", "a-bucket"),
			expectedErr:     "backupstoragelocations.velero.io \"a-location\" not found",
		},
		{
			name:            "backup contents request with phase '' gets a url",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindBackupContents, "a-backup"),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "backup contents request with phase 'New' gets a url",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseNew, velerov1api.DownloadTargetKindBackupContents, "a-backup"),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "backup log request with phase '' gets a url",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindBackupLog, "a-backup"),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "backup log request with phase 'New' gets a url",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseNew, velerov1api.DownloadTargetKindBackupLog, "a-backup"),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "restore log request with phase '' gets a url",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:         builder.ForRestore(velerov1api.DefaultNamespace, "a-backup-20170912150214").Phase(velerov1api.RestorePhaseCompleted).Backup("a-backup").Result(),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "restore log request with phase 'New' gets a url",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseNew, velerov1api.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:         builder.ForRestore(velerov1api.DefaultNamespace, "a-backup-20170912150214").Phase(velerov1api.RestorePhaseCompleted).Backup("a-backup").Result(),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "restore results request with phase '' gets a url",
			downloadRequest: newDownloadRequest("", velerov1api.DownloadTargetKindRestoreResults, "a-backup-20170912150214"),
			restore:         builder.ForRestore(velerov1api.DefaultNamespace, "a-backup-20170912150214").Phase(velerov1api.RestorePhaseCompleted).Backup("a-backup").Result(),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "restore results request with phase 'New' gets a url",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseNew, velerov1api.DownloadTargetKindRestoreResults, "a-backup-20170912150214"),
			restore:         builder.ForRestore(velerov1api.DefaultNamespace, "a-backup-20170912150214").Phase(velerov1api.RestorePhaseCompleted).Backup("a-backup").Result(),
			backup:          defaultBackup(),
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectGetsURL:   true,
		},
		{
			name:            "request with phase 'Processed' is not deleted if not expired",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseProcessed, velerov1api.DownloadTargetKindBackupLog, "a-backup-20170912150214"),
			backup:          defaultBackup(),
		},
		{
			name:            "request with phase 'Processed' is deleted if expired",
			downloadRequest: newDownloadRequest(velerov1api.DownloadRequestPhaseProcessed, velerov1api.DownloadTargetKindBackupLog, "a-backup-20170912150214"),
			backup:          defaultBackup(),
			expired:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var fakeClient client.Client
			if tc.backupLocation != nil {
				fakeClient = newFakeClient(t, tc.backupLocation)
			} else {
				fakeClient = newFakeClient(t)
			}

			harness := newDownloadRequestTestHarness(t, fakeClient)

			// set up test case data

			// Set .status.expiration properly for processed requests. Since "expired" is relative to the controller's
			// clock time, it's easier to do this here than as part of the test case definitions.
			if tc.downloadRequest != nil && tc.downloadRequest.Status.Phase == velerov1api.DownloadRequestPhaseProcessed {
				if tc.expired {
					tc.downloadRequest.Status.Expiration = &metav1.Time{Time: harness.controller.clock.Now().Add(-1 * time.Minute)}
				} else {
					tc.downloadRequest.Status.Expiration = &metav1.Time{Time: harness.controller.clock.Now().Add(time.Minute)}
				}
			}

			if tc.downloadRequest != nil {
				require.NoError(t, harness.informerFactory.Velero().V1().DownloadRequests().Informer().GetStore().Add(tc.downloadRequest))

				_, err := harness.client.VeleroV1().DownloadRequests(tc.downloadRequest.Namespace).Create(context.TODO(), tc.downloadRequest, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			if tc.restore != nil {
				require.NoError(t, harness.informerFactory.Velero().V1().Restores().Informer().GetStore().Add(tc.restore))
			}

			if tc.backup != nil {
				require.NoError(t, harness.informerFactory.Velero().V1().Backups().Informer().GetStore().Add(tc.backup))
			}

			if tc.expectGetsURL {
				harness.backupStore.On("GetDownloadURL", tc.downloadRequest.Spec.Target).Return("a-url", nil)
			}

			// exercise method under test
			key := tc.key
			if key == "" && tc.downloadRequest != nil {
				key = kubeutil.NamespaceAndName(tc.downloadRequest)
			}

			err := harness.controller.processDownloadRequest(key)

			// verify results
			if tc.expectedErr != "" {
				require.Equal(t, tc.expectedErr, err.Error())
			} else {
				assert.Nil(t, err)
			}

			if tc.expectGetsURL {
				output, err := harness.client.VeleroV1().DownloadRequests(tc.downloadRequest.Namespace).Get(context.TODO(), tc.downloadRequest.Name, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, string(velerov1api.DownloadRequestPhaseProcessed), string(output.Status.Phase))
				assert.Equal(t, "a-url", output.Status.DownloadURL)
				assert.True(t, velerotest.TimesAreEqual(harness.controller.clock.Now().Add(signedURLTTL), output.Status.Expiration.Time), "expiration does not match")
			}

			if tc.downloadRequest != nil && tc.downloadRequest.Status.Phase == velerov1api.DownloadRequestPhaseProcessed {
				res, err := harness.client.VeleroV1().DownloadRequests(tc.downloadRequest.Namespace).Get(context.TODO(), tc.downloadRequest.Name, metav1.GetOptions{})

				if tc.expired {
					assert.True(t, apierrors.IsNotFound(err))
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.downloadRequest, res)
				}
			}
		})
	}
}
