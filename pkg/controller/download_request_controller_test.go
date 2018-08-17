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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/plugin"
	pluginmocks "github.com/heptio/ark/pkg/plugin/mocks"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	arktest "github.com/heptio/ark/pkg/util/test"
)

type downloadRequestTestHarness struct {
	client          *fake.Clientset
	informerFactory informers.SharedInformerFactory
	pluginManager   *pluginmocks.Manager
	objectStore     *arktest.ObjectStore

	controller *downloadRequestController
}

func newDownloadRequestTestHarness(t *testing.T) *downloadRequestTestHarness {
	var (
		client          = fake.NewSimpleClientset()
		informerFactory = informers.NewSharedInformerFactory(client, 0)
		pluginManager   = new(pluginmocks.Manager)
		objectStore     = new(arktest.ObjectStore)
		controller      = NewDownloadRequestController(
			client.ArkV1(),
			informerFactory.Ark().V1().DownloadRequests(),
			informerFactory.Ark().V1().Restores(),
			informerFactory.Ark().V1().BackupStorageLocations(),
			informerFactory.Ark().V1().Backups(),
			nil,
			arktest.NewLogger(),
			logrus.InfoLevel,
		).(*downloadRequestController)
	)

	clockTime, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)

	controller.clock = clock.NewFakeClock(clockTime)

	controller.newPluginManager = func(_ logrus.FieldLogger) plugin.Manager { return pluginManager }

	pluginManager.On("CleanupClients").Return()
	objectStore.On("Init", mock.Anything).Return(nil)

	return &downloadRequestTestHarness{
		client:          client,
		informerFactory: informerFactory,
		pluginManager:   pluginManager,
		objectStore:     objectStore,
		controller:      controller,
	}
}

func newDownloadRequest(phase v1.DownloadRequestPhase, targetKind v1.DownloadTargetKind, targetName string) *v1.DownloadRequest {
	return &v1.DownloadRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-download-request",
			Namespace: v1.DefaultNamespace,
		},
		Spec: v1.DownloadRequestSpec{
			Target: v1.DownloadTarget{
				Kind: targetKind,
				Name: targetName,
			},
		},
		Status: v1.DownloadRequestStatus{
			Phase: phase,
		},
	}
}

func newBackupLocation(name, provider, bucket string) *v1.BackupStorageLocation {
	return &v1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: v1.DefaultNamespace,
		},
		Spec: v1.BackupStorageLocationSpec{
			Provider: provider,
			StorageType: v1.StorageType{
				ObjectStorage: &v1.ObjectStorageLocation{
					Bucket: bucket,
				},
			},
		},
	}
}

func TestProcessDownloadRequest(t *testing.T) {
	tests := []struct {
		name                    string
		key                     string
		downloadRequest         *v1.DownloadRequest
		backup                  *v1.Backup
		restore                 *v1.Restore
		backupLocation          *v1.BackupStorageLocation
		expired                 bool
		expectedErr             string
		expectedRequestedObject string
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
			downloadRequest: newDownloadRequest("", v1.DownloadTargetKindBackupContents, "a-backup"),
			backup:          arktest.NewTestBackup().WithName("non-matching-backup").WithStorageLocation("a-location").Backup,
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedErr:     "backup.ark.heptio.com \"a-backup\" not found",
		},
		{
			name:            "restore log request for nonexistent restore returns an error",
			downloadRequest: newDownloadRequest("", v1.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:         arktest.NewTestRestore(v1.DefaultNamespace, "non-matching-restore", v1.RestorePhaseCompleted).WithBackup("a-backup").Restore,
			backup:          arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:  newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedErr:     "error getting Restore: restore.ark.heptio.com \"a-backup-20170912150214\" not found",
		},
		{
			name:            "backup contents request for backup with nonexistent location returns an error",
			downloadRequest: newDownloadRequest("", v1.DownloadTargetKindBackupContents, "a-backup"),
			backup:          arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:  newBackupLocation("non-matching-location", "a-provider", "a-bucket"),
			expectedErr:     "backupstoragelocation.ark.heptio.com \"a-location\" not found",
		},
		{
			name:                    "backup contents request with phase '' gets a url",
			downloadRequest:         newDownloadRequest("", v1.DownloadTargetKindBackupContents, "a-backup"),
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/a-backup.tar.gz",
		},
		{
			name:                    "backup contents request with phase 'New' gets a url",
			downloadRequest:         newDownloadRequest(v1.DownloadRequestPhaseNew, v1.DownloadTargetKindBackupContents, "a-backup"),
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/a-backup.tar.gz",
		},
		{
			name:                    "backup log request with phase '' gets a url",
			downloadRequest:         newDownloadRequest("", v1.DownloadTargetKindBackupLog, "a-backup"),
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/a-backup-logs.gz",
		},
		{
			name:                    "backup log request with phase 'New' gets a url",
			downloadRequest:         newDownloadRequest(v1.DownloadRequestPhaseNew, v1.DownloadTargetKindBackupLog, "a-backup"),
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/a-backup-logs.gz",
		},
		{
			name:                    "restore log request with phase '' gets a url",
			downloadRequest:         newDownloadRequest("", v1.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:                 arktest.NewTestRestore(v1.DefaultNamespace, "a-backup-20170912150214", v1.RestorePhaseCompleted).WithBackup("a-backup").Restore,
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/restore-a-backup-20170912150214-logs.gz",
		},
		{
			name:                    "restore log request with phase 'New' gets a url",
			downloadRequest:         newDownloadRequest(v1.DownloadRequestPhaseNew, v1.DownloadTargetKindRestoreLog, "a-backup-20170912150214"),
			restore:                 arktest.NewTestRestore(v1.DefaultNamespace, "a-backup-20170912150214", v1.RestorePhaseCompleted).WithBackup("a-backup").Restore,
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/restore-a-backup-20170912150214-logs.gz",
		},
		{
			name:                    "restore results request with phase '' gets a url",
			downloadRequest:         newDownloadRequest("", v1.DownloadTargetKindRestoreResults, "a-backup-20170912150214"),
			restore:                 arktest.NewTestRestore(v1.DefaultNamespace, "a-backup-20170912150214", v1.RestorePhaseCompleted).WithBackup("a-backup").Restore,
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/restore-a-backup-20170912150214-results.gz",
		},
		{
			name:                    "restore results request with phase 'New' gets a url",
			downloadRequest:         newDownloadRequest(v1.DownloadRequestPhaseNew, v1.DownloadTargetKindRestoreResults, "a-backup-20170912150214"),
			restore:                 arktest.NewTestRestore(v1.DefaultNamespace, "a-backup-20170912150214", v1.RestorePhaseCompleted).WithBackup("a-backup").Restore,
			backup:                  arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			backupLocation:          newBackupLocation("a-location", "a-provider", "a-bucket"),
			expectedRequestedObject: "a-backup/restore-a-backup-20170912150214-results.gz",
		},
		{
			name:            "request with phase 'Processed' is not deleted if not expired",
			downloadRequest: newDownloadRequest(v1.DownloadRequestPhaseProcessed, v1.DownloadTargetKindBackupLog, "a-backup-20170912150214"),
			backup:          arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
		},
		{
			name:            "request with phase 'Processed' is deleted if expired",
			downloadRequest: newDownloadRequest(v1.DownloadRequestPhaseProcessed, v1.DownloadTargetKindBackupLog, "a-backup-20170912150214"),
			backup:          arktest.NewTestBackup().WithName("a-backup").WithStorageLocation("a-location").Backup,
			expired:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newDownloadRequestTestHarness(t)

			// set up test case data

			// Set .status.expiration properly for processed requests. Since "expired" is relative to the controller's
			// clock time, it's easier to do this here than as part of the test case definitions.
			if tc.downloadRequest != nil && tc.downloadRequest.Status.Phase == v1.DownloadRequestPhaseProcessed {
				if tc.expired {
					tc.downloadRequest.Status.Expiration.Time = harness.controller.clock.Now().Add(-1 * time.Minute)
				} else {
					tc.downloadRequest.Status.Expiration.Time = harness.controller.clock.Now().Add(time.Minute)
				}
			}

			if tc.downloadRequest != nil {
				require.NoError(t, harness.informerFactory.Ark().V1().DownloadRequests().Informer().GetStore().Add(tc.downloadRequest))

				_, err := harness.client.ArkV1().DownloadRequests(tc.downloadRequest.Namespace).Create(tc.downloadRequest)
				require.NoError(t, err)
			}

			if tc.restore != nil {
				require.NoError(t, harness.informerFactory.Ark().V1().Restores().Informer().GetStore().Add(tc.restore))
			}

			if tc.backup != nil {
				require.NoError(t, harness.informerFactory.Ark().V1().Backups().Informer().GetStore().Add(tc.backup))
			}

			if tc.backupLocation != nil {
				require.NoError(t, harness.informerFactory.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(tc.backupLocation))

				harness.pluginManager.On("GetObjectStore", tc.backupLocation.Spec.Provider).Return(harness.objectStore, nil)
			}

			if tc.expectedRequestedObject != "" {
				harness.objectStore.On("CreateSignedURL", tc.backupLocation.Spec.ObjectStorage.Bucket, tc.expectedRequestedObject, mock.Anything).Return("a-url", nil)
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

			if tc.expectedRequestedObject != "" {
				output, err := harness.client.ArkV1().DownloadRequests(tc.downloadRequest.Namespace).Get(tc.downloadRequest.Name, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, string(v1.DownloadRequestPhaseProcessed), string(output.Status.Phase))
				assert.Equal(t, "a-url", output.Status.DownloadURL)
				assert.True(t, arktest.TimesAreEqual(harness.controller.clock.Now().Add(signedURLTTL), output.Status.Expiration.Time), "expiration does not match")
			}

			if tc.downloadRequest != nil && tc.downloadRequest.Status.Phase == v1.DownloadRequestPhaseProcessed {
				res, err := harness.client.ArkV1().DownloadRequests(tc.downloadRequest.Namespace).Get(tc.downloadRequest.Name, metav1.GetOptions{})

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
