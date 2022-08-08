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

package provider

import (
	"context"
	"testing"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/kopia"
)

func TestRunBackup(t *testing.T) {
	var kp kopiaProvider
	kp.log = logrus.New()
	fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme)
	updater := velerov1api.NewBackupProgressUpdater(&velerov1api.PodVolumeBackup{}, kp.log, context.Background(), fakeClient)
	testCases := []struct {
		name           string
		hookBackupFunc func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, error)
		notError       bool
	}{
		{
			name: "success to backup",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, error) {
				return &uploader.SnapshotInfo{}, nil
			},
			notError: true,
		},
		{
			name: "get error to backup",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, error) {
				return &uploader.SnapshotInfo{}, errors.New("failed to backup")
			},
			notError: false,
		},
		{
			name: "got empty snapshot",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, error) {
				return nil, nil
			},
			notError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			BackupFunc = tc.hookBackupFunc
			_, err := kp.RunBackup(context.Background(), "var", nil, "", updater)
			if tc.notError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestRunRestore(t *testing.T) {
	var kp kopiaProvider
	kp.log = logrus.New()
	updater := velerov1api.NewRestoreProgressUpdater(&velerov1api.PodVolumeRestore{}, kp.log, context.Background(), fake.NewFakeClientWithScheme(scheme.Scheme))

	testCases := []struct {
		name            string
		hookRestoreFunc func(ctx context.Context, rep repo.RepositoryWriter, progress *kopia.KopiaProgress, snapshotID, dest string, log logrus.FieldLogger, cancleCh chan struct{}) (int64, int32, error)
		notError        bool
	}{
		{
			name: "normal restore",
			hookRestoreFunc: func(ctx context.Context, rep repo.RepositoryWriter, progress *kopia.KopiaProgress, snapshotID, dest string, log logrus.FieldLogger, cancleCh chan struct{}) (int64, int32, error) {
				return 0, 0, nil
			},
			notError: true,
		},
		{
			name: "failed to restore",
			hookRestoreFunc: func(ctx context.Context, rep repo.RepositoryWriter, progress *kopia.KopiaProgress, snapshotID, dest string, log logrus.FieldLogger, cancleCh chan struct{}) (int64, int32, error) {
				return 0, 0, errors.New("failed to restore")
			},
			notError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RestoreFunc = tc.hookRestoreFunc
			err := kp.RunRestore(context.Background(), "", "/var", updater)
			if tc.notError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
