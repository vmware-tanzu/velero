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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/kopia"
)

func TestRunBackup(t *testing.T) {
	var kp kopiaProvider
	kp.log = logrus.New()
	updater := FakeBackupProgressUpdater{PodVolumeBackup: &velerov1api.PodVolumeBackup{}, Log: kp.log, Ctx: context.Background(), Cli: fake.NewFakeClientWithScheme(scheme.Scheme)}
	testCases := []struct {
		name           string
		hookBackupFunc func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, bool, error)
		notError       bool
	}{
		{
			name: "success to backup",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, bool, error) {
				return &uploader.SnapshotInfo{}, false, nil
			},
			notError: true,
		},
		{
			name: "get error to backup",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, bool, error) {
				return &uploader.SnapshotInfo{}, false, errors.New("failed to backup")
			},
			notError: false,
		},
		{
			name: "got empty snapshot",
			hookBackupFunc: func(ctx context.Context, fsUploader *snapshotfs.Uploader, repoWriter repo.RepositoryWriter, sourcePath, parentSnapshot string, log logrus.FieldLogger) (*uploader.SnapshotInfo, bool, error) {
				return nil, true, errors.New("snapshot is empty")
			},
			notError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			BackupFunc = tc.hookBackupFunc
			_, _, err := kp.RunBackup(context.Background(), "var", nil, "", &updater)
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
	updater := FakeRestoreProgressUpdater{PodVolumeRestore: &velerov1api.PodVolumeRestore{}, Log: kp.log, Ctx: context.Background(), Cli: fake.NewFakeClientWithScheme(scheme.Scheme)}

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
			err := kp.RunRestore(context.Background(), "", "/var", &updater)
			if tc.notError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

type FakeBackupProgressUpdater struct {
	PodVolumeBackup *velerov1api.PodVolumeBackup
	Log             logrus.FieldLogger
	Ctx             context.Context
	Cli             client.Client
}

func (f *FakeBackupProgressUpdater) UpdateProgress(p *uploader.UploaderProgress) {}

type FakeRestoreProgressUpdater struct {
	PodVolumeRestore *velerov1api.PodVolumeRestore
	Log              logrus.FieldLogger
	Ctx              context.Context
	Cli              client.Client
}

func (f *FakeRestoreProgressUpdater) UpdateProgress(p *uploader.UploaderProgress) {}
