/*
Copyright 2017, 2019 the Velero contributors.

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
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer, actions []velero.BackupItemAction, volumeSnapshotterGetter pkgbackup.VolumeSnapshotterGetter) error {
	args := b.Called(logger, backup, backupFile, actions, volumeSnapshotterGetter)
	return args.Error(0)
}

func defaultBackup() *builder.BackupBuilder {
	return builder.ForBackup(velerov1api.DefaultNamespace, "backup-1")
}

func TestProcessBackupNonProcessedItems(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		backup *velerov1api.Backup
	}{
		{
			name: "bad key does not return error",
			key:  "bad/key/here",
		},
		{
			name: "backup not found in lister does not return error",
			key:  "nonexistent/backup",
		},
		{
			name:   "FailedValidation backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailedValidation).Result(),
		},
		{
			name:   "InProgress backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseInProgress).Result(),
		},
		{
			name:   "Completed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseCompleted).Result(),
		},
		{
			name:   "Failed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailed).Result(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				sharedInformers = informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			c := &backupController{
				genericController: newGenericController("backup-test", logger),
				lister:            sharedInformers.Velero().V1().Backups().Lister(),
			}

			if test.backup != nil {
				require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))
			}

			err := c.processBackup(test.key)
			assert.Nil(t, err)

			// Any backup that would actually proceed to validation will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for validation/etc. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

// Test_getLastSuccessBySchedule verifies that the getLastSuccessBySchedule helper function correctly returns
// the completion timestamp of the most recent completed backup for each schedule, including an entry for ad-hoc
// or non-scheduled backups.
func Test_getLastSuccessBySchedule(t *testing.T) {
	buildBackup := func(phase velerov1api.BackupPhase, completion time.Time, schedule string) *velerov1api.Backup {
		b := builder.ForBackup("", "").
			ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, schedule)).
			Phase(phase)

		if !completion.IsZero() {
			b.CompletionTimestamp(completion)
		}

		return b.Result()
	}

	// create a static "base time" that can be used to easily construct completion timestamps
	// by using the .Add(...) method.
	baseTime, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)

	tests := []struct {
		name    string
		backups []*velerov1api.Backup
		want    map[string]time.Time
	}{
		{
			name:    "when backups is nil, an empty map is returned",
			backups: nil,
			want:    map[string]time.Time{},
		},
		{
			name:    "when backups is empty, an empty map is returned",
			backups: []*velerov1api.Backup{},
			want:    map[string]time.Time{},
		},
		{
			name: "when multiple completed backups for a schedule exist, the latest one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime.Add(time.Second),
			},
		},
		{
			name: "when the most recent backup for a schedule is Failed, the timestamp of the most recent Completed one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime,
			},
		},
		{
			name: "when there are no Completed backups for a schedule, it's not returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{},
		},
		{
			name: "when backups exist without a schedule, the most recent Completed one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),
			},
			want: map[string]time.Time{
				"": baseTime,
			},
		},
		{
			name: "when backups exist for multiple schedules, the most recent Completed timestamp for each schedule is returned",
			backups: []*velerov1api.Backup{
				// ad-hoc backups (no schedule)
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(30*time.Minute), ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Hour), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),

				// schedule-1
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),

				// schedule-2
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(24*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(48*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(72*time.Hour), "schedule-2"),

				// schedule-3
				buildBackup(velerov1api.BackupPhaseNew, baseTime, "schedule-3"),
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime.Add(time.Minute), "schedule-3"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(2*time.Minute), "schedule-3"),
			},
			want: map[string]time.Time{
				"":           baseTime.Add(30 * time.Minute),
				"schedule-1": baseTime,
				"schedule-2": baseTime.Add(72 * time.Hour),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, getLastSuccessBySchedule(tc.backups))
		})
	}
}
