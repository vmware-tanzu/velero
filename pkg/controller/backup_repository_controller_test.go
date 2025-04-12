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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/repository/maintenance"
	repomokes "github.com/vmware-tanzu/velero/pkg/repository/mocks"
	repotypes "github.com/vmware-tanzu/velero/pkg/repository/types"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"

	"sigs.k8s.io/controller-runtime/pkg/client"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	batchv1 "k8s.io/api/batch/v1"
)

const testMaintenanceFrequency = 10 * time.Minute

func mockBackupRepoReconciler(t *testing.T, mockOn string, arg any, ret ...any) *BackupRepoReconciler {
	t.Helper()
	mgr := &repomokes.Manager{}
	if mockOn != "" {
		mgr.On(mockOn, arg).Return(ret...)
	}
	return NewBackupRepoReconciler(
		velerov1api.DefaultNamespace,
		velerotest.NewLogger(),
		velerotest.NewFakeControllerRuntimeClient(t),
		mgr,
		testMaintenanceFrequency,
		"fake-repo-config",
		3,
		"",
		kube.PodResources{},
		logrus.InfoLevel,
		nil,
	)
}

func mockBackupRepositoryCR() *velerov1api.BackupRepository {
	return &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Spec: velerov1api.BackupRepositorySpec{
			MaintenanceFrequency: metav1.Duration{Duration: testMaintenanceFrequency},
		},
	}
}

func TestPatchBackupRepository(t *testing.T) {
	rr := mockBackupRepositoryCR()
	reconciler := mockBackupRepoReconciler(t, "", nil, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	err = reconciler.patchBackupRepository(context.Background(), rr, repoReady())
	assert.NoError(t, err)
	assert.Equal(t, velerov1api.BackupRepositoryPhaseReady, rr.Status.Phase)
	err = reconciler.patchBackupRepository(context.Background(), rr, repoNotReady("not ready"))
	assert.NoError(t, err)
	assert.NotEqual(t, velerov1api.BackupRepositoryPhaseReady, rr.Status.Phase)
}

func TestCheckNotReadyRepo(t *testing.T) {
	rr := mockBackupRepositoryCR()
	rr.Spec.BackupStorageLocation = "default"
	rr.Spec.ResticIdentifier = "fake-identifier"
	rr.Spec.VolumeNamespace = "volume-ns-1"
	reconciler := mockBackupRepoReconciler(t, "PrepareRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	location := velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Config: map[string]string{"resticRepoPrefix": "s3:test.amazonaws.com/bucket/restic"},
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      rr.Spec.BackupStorageLocation,
		},
	}

	_, err = reconciler.checkNotReadyRepo(context.TODO(), rr, &location, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, velerov1api.BackupRepositoryPhaseReady, rr.Status.Phase)
	assert.Equal(t, "s3:test.amazonaws.com/bucket/restic/volume-ns-1", rr.Spec.ResticIdentifier)
}

func startMaintenanceJobFail(client.Client, context.Context, *velerov1api.BackupRepository, string, kube.PodResources, logrus.Level, *logging.FormatFlag, logrus.FieldLogger) (string, error) {
	return "", errors.New("fake-start-error")
}

func startMaintenanceJobSucceed(client.Client, context.Context, *velerov1api.BackupRepository, string, kube.PodResources, logrus.Level, *logging.FormatFlag, logrus.FieldLogger) (string, error) {
	return "fake-job-name", nil
}

func waitMaintenanceJobCompleteFail(client.Client, context.Context, string, string, logrus.FieldLogger) (velerov1api.BackupRepositoryMaintenanceStatus, error) {
	return velerov1api.BackupRepositoryMaintenanceStatus{}, errors.New("fake-wait-error")
}

func waitMaintenanceJobCompleteFunc(now time.Time, result velerov1api.BackupRepositoryMaintenanceResult, message string) func(client.Client, context.Context, string, string, logrus.FieldLogger) (velerov1api.BackupRepositoryMaintenanceStatus, error) {
	completionTimeStamp := &metav1.Time{Time: now.Add(time.Hour)}
	if result == velerov1api.BackupRepositoryMaintenanceFailed {
		completionTimeStamp = nil
	}

	return func(client.Client, context.Context, string, string, logrus.FieldLogger) (velerov1api.BackupRepositoryMaintenanceStatus, error) {
		return velerov1api.BackupRepositoryMaintenanceStatus{
			StartTimestamp:    &metav1.Time{Time: now},
			CompleteTimestamp: completionTimeStamp,
			Result:            result,
			Message:           message,
		}, nil
	}
}

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) After(time.Duration) <-chan time.Time {
	return nil
}

func (f *fakeClock) NewTicker(time.Duration) clock.Ticker {
	return nil
}
func (f *fakeClock) NewTimer(time.Duration) clock.Timer {
	return nil
}

func (f *fakeClock) Now() time.Time {
	return f.now
}

func (f *fakeClock) Since(time.Time) time.Duration {
	return 0
}

func (f *fakeClock) Sleep(time.Duration) {}

func (f *fakeClock) Tick(time.Duration) <-chan time.Time {
	return nil
}

func (f *fakeClock) AfterFunc(time.Duration, func()) clock.Timer {
	return nil
}

func TestRunMaintenanceIfDue(t *testing.T) {
	now := time.Now().Round(time.Second)

	tests := []struct {
		name                    string
		repo                    *velerov1api.BackupRepository
		startJobFunc            func(client.Client, context.Context, *velerov1api.BackupRepository, string, kube.PodResources, logrus.Level, *logging.FormatFlag, logrus.FieldLogger) (string, error)
		waitJobFunc             func(client.Client, context.Context, string, string, logrus.FieldLogger) (velerov1api.BackupRepositoryMaintenanceStatus, error)
		expectedMaintenanceTime time.Time
		expectedHistory         []velerov1api.BackupRepositoryMaintenanceStatus
		expectedErr             string
	}{
		{
			name: "not due",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: time.Hour},
				},
				Status: velerov1api.BackupRepositoryStatus{
					LastMaintenanceTime: &metav1.Time{Time: now},
					RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
						{
							StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour)},
							CompleteTimestamp: &metav1.Time{Time: now},
							Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
						},
					},
				},
			},
			expectedMaintenanceTime: now,
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
			},
		},
		{
			name: "start failed",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: time.Hour},
				},
				Status: velerov1api.BackupRepositoryStatus{
					LastMaintenanceTime: &metav1.Time{Time: now.Add(-time.Hour - time.Minute)},
					RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
						{
							StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
							CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
							Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
						},
					},
				},
			},
			startJobFunc:            startMaintenanceJobFail,
			expectedMaintenanceTime: now.Add(-time.Hour - time.Minute),
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
				{
					StartTimestamp: &metav1.Time{Time: now},
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					Message:        "Failed to start maintenance job, err: fake-start-error",
				},
			},
		},
		{
			name: "wait failed",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: time.Hour},
				},
				Status: velerov1api.BackupRepositoryStatus{
					LastMaintenanceTime: &metav1.Time{Time: now.Add(-time.Hour - time.Minute)},
					RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
						{
							StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
							CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
							Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
						},
					},
				},
			},
			startJobFunc:            startMaintenanceJobSucceed,
			waitJobFunc:             waitMaintenanceJobCompleteFail,
			expectedErr:             "error waiting repo maintenance completion status: fake-wait-error",
			expectedMaintenanceTime: now.Add(-time.Hour - time.Minute),
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
			},
		},
		{
			name: "maintenance failed",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: time.Hour},
				},
				Status: velerov1api.BackupRepositoryStatus{
					LastMaintenanceTime: &metav1.Time{Time: now.Add(-time.Hour - time.Minute)},
					RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
						{
							StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
							CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
							Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
						},
					},
				},
			},
			startJobFunc:            startMaintenanceJobSucceed,
			waitJobFunc:             waitMaintenanceJobCompleteFunc(now, velerov1api.BackupRepositoryMaintenanceFailed, "fake-maintenance-message"),
			expectedMaintenanceTime: now.Add(-time.Hour - time.Minute),
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
				{
					StartTimestamp: &metav1.Time{Time: now},
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					Message:        "fake-maintenance-message",
				},
			},
		},
		{
			name: "maintenance succeeded",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: time.Hour},
				},
				Status: velerov1api.BackupRepositoryStatus{
					LastMaintenanceTime: &metav1.Time{Time: now.Add(-time.Hour - time.Minute)},
					RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
						{
							StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
							CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
							Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
						},
					},
				},
			},
			startJobFunc:            startMaintenanceJobSucceed,
			waitJobFunc:             waitMaintenanceJobCompleteFunc(now, velerov1api.BackupRepositoryMaintenanceSucceeded, ""),
			expectedMaintenanceTime: now.Add(time.Hour),
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(-time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(-time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := mockBackupRepoReconciler(t, "", test.repo, nil)
			reconciler.clock = &fakeClock{now}
			err := reconciler.Client.Create(context.TODO(), test.repo)
			assert.NoError(t, err)

			funcStartMaintenanceJob = test.startJobFunc
			funcWaitMaintenanceJobComplete = test.waitJobFunc

			err = reconciler.runMaintenanceIfDue(context.TODO(), test.repo, velerotest.NewLogger())
			if test.expectedErr == "" {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedMaintenanceTime, test.repo.Status.LastMaintenanceTime.Time)
			assert.Len(t, test.repo.Status.RecentMaintenance, len(test.expectedHistory))

			for i := 0; i < len(test.expectedHistory); i++ {
				assert.Equal(t, test.expectedHistory[i].StartTimestamp.Time, test.repo.Status.RecentMaintenance[i].StartTimestamp.Time)
				if test.expectedHistory[i].CompleteTimestamp == nil {
					assert.Nil(t, test.repo.Status.RecentMaintenance[i].CompleteTimestamp)
				} else {
					assert.Equal(t, test.expectedHistory[i].CompleteTimestamp.Time, test.repo.Status.RecentMaintenance[i].CompleteTimestamp.Time)
				}

				assert.Equal(t, test.expectedHistory[i].Result, test.repo.Status.RecentMaintenance[i].Result)
				assert.Equal(t, test.expectedHistory[i].Message, test.repo.Status.RecentMaintenance[i].Message)
			}
		})
	}
}

func TestInitializeRepo(t *testing.T) {
	rr := mockBackupRepositoryCR()
	rr.Spec.BackupStorageLocation = "default"
	reconciler := mockBackupRepoReconciler(t, "PrepareRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	location := velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Config: map[string]string{"resticRepoPrefix": "s3:test.amazonaws.com/bucket/restic"},
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      rr.Spec.BackupStorageLocation,
		},
	}

	err = reconciler.initializeRepo(context.TODO(), rr, &location, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, velerov1api.BackupRepositoryPhaseReady, rr.Status.Phase)
}

func TestBackupRepoReconcile(t *testing.T) {
	tests := []struct {
		name      string
		repo      *velerov1api.BackupRepository
		expectNil bool
	}{
		{
			name: "test on api server not found",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "unknown",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: testMaintenanceFrequency},
				},
			},
			expectNil: true,
		},
		{
			name: "test on initialize repo",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: testMaintenanceFrequency},
				},
			},
			expectNil: true,
		},
		{
			name: "test on repo with new phase",
			repo: &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.BackupRepositorySpec{
					MaintenanceFrequency: metav1.Duration{Duration: testMaintenanceFrequency},
				},
				Status: velerov1api.BackupRepositoryStatus{
					Phase: velerov1api.BackupRepositoryPhaseNew,
				},
			},
			expectNil: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := mockBackupRepoReconciler(t, "", test.repo, nil)
			err := reconciler.Client.Create(context.TODO(), test.repo)
			assert.NoError(t, err)
			_, err = reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.repo.Namespace, Name: "repo"}})
			if test.expectNil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetRepositoryMaintenanceFrequency(t *testing.T) {
	tests := []struct {
		name            string
		mgr             repotypes.SnapshotIdentifier
		repo            *velerov1api.BackupRepository
		freqReturn      time.Duration
		freqError       error
		userDefinedFreq time.Duration
		expectFreq      time.Duration
	}{
		{
			name:            "user defined valid",
			userDefinedFreq: time.Hour,
			expectFreq:      time.Hour,
		},
		{
			name:       "repo return valid",
			freqReturn: time.Hour * 2,
			expectFreq: time.Hour * 2,
		},
		{
			name:            "fall to default",
			userDefinedFreq: -1,
			freqError:       errors.New("fake-error"),
			expectFreq:      defaultMaintainFrequency,
		},
		{
			name:       "fall to default, no freq error",
			freqReturn: -1,
			expectFreq: defaultMaintainFrequency,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr := repomokes.Manager{}
			mgr.On("DefaultMaintenanceFrequency", mock.Anything).Return(test.freqReturn, test.freqError)
			reconciler := NewBackupRepoReconciler(
				velerov1api.DefaultNamespace,
				velerotest.NewLogger(),
				velerotest.NewFakeControllerRuntimeClient(t),
				&mgr,
				test.userDefinedFreq,
				"",
				3,
				"",
				kube.PodResources{},
				logrus.InfoLevel,
				nil,
			)

			freq := reconciler.getRepositoryMaintenanceFrequency(test.repo)
			assert.Equal(t, test.expectFreq, freq)
		})
	}
}

func TestNeedInvalidBackupRepo(t *testing.T) {
	tests := []struct {
		name   string
		oldBSL *velerov1api.BackupStorageLocation
		newBSL *velerov1api.BackupStorageLocation
		expect bool
	}{
		{
			name: "no change",
			oldBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "old-provider",
				},
			},
			newBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "new-provider",
				},
			},
			expect: false,
		},
		{
			name:   "other part change",
			oldBSL: &velerov1api.BackupStorageLocation{},
			newBSL: &velerov1api.BackupStorageLocation{},
			expect: false,
		},
		{
			name: "bucket change",
			oldBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "old-bucket",
						},
					},
				},
			},
			newBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "new-bucket",
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "prefix change",
			oldBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Prefix: "old-prefix",
						},
					},
				},
			},
			newBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Prefix: "new-prefix",
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "CACert change",
			oldBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							CACert: []byte{0x11, 0x12, 0x13},
						},
					},
				},
			},
			newBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							CACert: []byte{0x21, 0x22, 0x23},
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "config change",
			oldBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Config: map[string]string{
						"key1": "value1",
					},
				},
			},
			newBSL: &velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Config: map[string]string{
						"key2": "value2",
					},
				},
			},
			expect: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := NewBackupRepoReconciler(
				velerov1api.DefaultNamespace,
				velerotest.NewLogger(),
				velerotest.NewFakeControllerRuntimeClient(t),
				nil,
				time.Duration(0),
				"",
				3,
				"",
				kube.PodResources{},
				logrus.InfoLevel,
				nil)

			need := reconciler.needInvalidBackupRepo(test.oldBSL, test.newBSL)
			assert.Equal(t, test.expect, need)
		})
	}
}

func TestGetBackupRepositoryConfig(t *testing.T) {
	configWithNoData := &corev1api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-1",
			Namespace: velerov1api.DefaultNamespace,
		},
	}

	configWithWrongData := &corev1api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-1",
			Namespace: velerov1api.DefaultNamespace,
		},
		Data: map[string]string{
			"fake-repo-type": "",
		},
	}

	configWithData := &corev1api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-1",
			Namespace: velerov1api.DefaultNamespace,
		},
		Data: map[string]string{
			"fake-repo-type":   "{\"cacheLimitMB\": 1000, \"enableCompression\": true, \"fullMaintenanceInterval\": \"fastGC\"}",
			"fake-repo-type-1": "{\"cacheLimitMB\": 1, \"enableCompression\": false}",
		},
	}

	tests := []struct {
		name           string
		congiName      string
		repoName       string
		repoType       string
		kubeClientObj  []runtime.Object
		expectedErr    string
		expectedResult map[string]string
	}{
		{
			name: "empty configName",
		},
		{
			name:        "get error",
			congiName:   "config-1",
			expectedErr: "error getting configMap config-1: configmaps \"config-1\" not found",
		},
		{
			name:      "no config for repo",
			congiName: "config-1",
			repoName:  "fake-repo",
			repoType:  "fake-repo-type",
			kubeClientObj: []runtime.Object{
				configWithNoData,
			},
		},
		{
			name:      "unmarshall error",
			congiName: "config-1",
			repoName:  "fake-repo",
			repoType:  "fake-repo-type",
			kubeClientObj: []runtime.Object{
				configWithWrongData,
			},
			expectedErr: "error unmarshalling config data from config-1 for repo fake-repo, repo type fake-repo-type: unexpected end of JSON input",
		},
		{
			name:      "succeed",
			congiName: "config-1",
			repoName:  "fake-repo",
			repoType:  "fake-repo-type",
			kubeClientObj: []runtime.Object{
				configWithData,
			},
			expectedResult: map[string]string{
				"cacheLimitMB":            "1000",
				"enableCompression":       "true",
				"fullMaintenanceInterval": "fastGC",
			},
		},
	}

	scheme := runtime.NewScheme()
	corev1api.AddToScheme(scheme)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClientBuilder := clientFake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(scheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			result, err := getBackupRepositoryConfig(context.Background(), fakeClient, test.congiName, velerov1api.DefaultNamespace, test.repoName, test.repoType, velerotest.NewLogger())

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}

func TestUpdateRepoMaintenanceHistory(t *testing.T) {
	standardTime := time.Now()

	backupRepoWithoutHistory := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
	}

	backupRepoWithHistory := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Status: velerov1api.BackupRepositoryStatus{
			RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 24)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 23)},
					Message:           "fake-history-message-1",
				},
			},
		},
	}

	backupRepoWithFullHistory := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Status: velerov1api.BackupRepositoryStatus{
			RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 24)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 23)},
					Message:           "fake-history-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 22)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 21)},
					Message:           "fake-history-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 20)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 19)},
					Message:           "fake-history-message-4",
				},
			},
		},
	}

	backupRepoWithOverFullHistory := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Status: velerov1api.BackupRepositoryStatus{
			RecentMaintenance: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 24)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 23)},
					Message:           "fake-history-message-5",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 22)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 21)},
					Message:           "fake-history-message-6",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 20)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 19)},
					Message:           "fake-history-message-7",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 18)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 17)},
					Message:           "fake-history-message-8",
				},
			},
		},
	}

	tests := []struct {
		name            string
		backupRepo      *velerov1api.BackupRepository
		result          velerov1api.BackupRepositoryMaintenanceResult
		expectedHistory []velerov1api.BackupRepositoryMaintenanceStatus
	}{
		{
			name:       "empty history",
			backupRepo: backupRepoWithoutHistory,
			result:     velerov1api.BackupRepositoryMaintenanceSucceeded,
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(time.Hour)},
					Message:           "fake-message-0",
				},
			},
		},
		{
			name:       "less than history queue length",
			backupRepo: backupRepoWithHistory,
			result:     velerov1api.BackupRepositoryMaintenanceSucceeded,
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 24)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 23)},
					Message:           "fake-history-message-1",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(time.Hour)},
					Message:           "fake-message-0",
				},
			},
		},
		{
			name:       "full history",
			backupRepo: backupRepoWithFullHistory,
			result:     velerov1api.BackupRepositoryMaintenanceSucceeded,
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 22)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 21)},
					Message:           "fake-history-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 20)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 19)},
					Message:           "fake-history-message-4",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(time.Hour)},
					Message:           "fake-message-0",
				},
			},
		},
		{
			name:       "over full history",
			backupRepo: backupRepoWithOverFullHistory,
			result:     velerov1api.BackupRepositoryMaintenanceSucceeded,
			expectedHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 20)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 19)},
					Message:           "fake-history-message-7",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime.Add(-time.Hour * 18)},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(-time.Hour * 17)},
					Message:           "fake-history-message-8",
				},
				{
					StartTimestamp:    &metav1.Time{Time: standardTime},
					CompleteTimestamp: &metav1.Time{Time: standardTime.Add(time.Hour)},
					Message:           "fake-message-0",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updateRepoMaintenanceHistory(test.backupRepo, test.result, &metav1.Time{Time: standardTime}, &metav1.Time{Time: standardTime.Add(time.Hour)}, "fake-message-0")

			for at := range test.backupRepo.Status.RecentMaintenance {
				assert.Equal(t, test.expectedHistory[at].StartTimestamp.Time, test.backupRepo.Status.RecentMaintenance[at].StartTimestamp.Time)
				assert.Equal(t, test.expectedHistory[at].CompleteTimestamp.Time, test.backupRepo.Status.RecentMaintenance[at].CompleteTimestamp.Time)
				assert.Equal(t, test.expectedHistory[at].Message, test.backupRepo.Status.RecentMaintenance[at].Message)
			}
		})
	}
}

func TestRecallMaintenance(t *testing.T) {
	now := time.Now().Round(time.Second)

	schemeFail := runtime.NewScheme()
	velerov1api.AddToScheme(schemeFail)

	scheme := runtime.NewScheme()
	batchv1.AddToScheme(scheme)
	corev1api.AddToScheme(scheme)
	velerov1api.AddToScheme(scheme)

	jobSucceeded := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job1",
			Namespace:         velerov1api.DefaultNamespace,
			Labels:            map[string]string{maintenance.RepositoryNameLabel: "repo"},
			CreationTimestamp: metav1.Time{Time: now.Add(time.Hour)},
		},
		Status: batchv1.JobStatus{
			StartTime:      &metav1.Time{Time: now.Add(time.Hour)},
			CompletionTime: &metav1.Time{Time: now.Add(time.Hour * 2)},
			Succeeded:      1,
		},
	}

	jobPodSucceeded := builder.ForPod(velerov1api.DefaultNamespace, "job1").Labels(map[string]string{"job-name": "job1"}).ContainerStatuses(&corev1api.ContainerStatus{
		State: corev1api.ContainerState{
			Terminated: &corev1api.ContainerStateTerminated{},
		},
	}).Result()

	tests := []struct {
		name               string
		kubeClientObj      []runtime.Object
		runtimeScheme      *runtime.Scheme
		repoLastMatainTime metav1.Time
		expectNewHistory   []velerov1api.BackupRepositoryMaintenanceStatus
		expectTimeUpdate   *metav1.Time
		expectedErr        string
	}{
		{
			name:          "wait completion error",
			runtimeScheme: schemeFail,
			expectedErr:   "error waiting incomplete repo maintenance job for repo repo: error listing maintenance job for repo repo: no kind is registered for the type v1.JobList in scheme \"pkg/runtime/scheme.go:100\"",
		},
		{
			name:          "no consolidate result",
			runtimeScheme: scheme,
		},
		{
			name:          "no update last time",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded,
				jobPodSucceeded,
			},
			repoLastMatainTime: metav1.Time{Time: now.Add(time.Hour * 5)},
			expectNewHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
			},
		},
		{
			name:          "update last time",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded,
				jobPodSucceeded,
			},
			expectNewHistory: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
				},
			},
			expectTimeUpdate: &metav1.Time{Time: now.Add(time.Hour * 2)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := mockBackupRepoReconciler(t, "", nil, nil)

			backupRepo := mockBackupRepositoryCR()
			backupRepo.Status.LastMaintenanceTime = &test.repoLastMatainTime

			test.kubeClientObj = append(test.kubeClientObj, backupRepo)

			fakeClientBuilder := clientFake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(test.runtimeScheme)
			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()
			r.Client = fakeClient

			lastTm := backupRepo.Status.LastMaintenanceTime

			err := r.recallMaintenance(context.TODO(), backupRepo, velerotest.NewLogger())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)

				if test.expectNewHistory == nil {
					assert.Nil(t, backupRepo.Status.RecentMaintenance)
				} else {
					assert.Len(t, backupRepo.Status.RecentMaintenance, len(test.expectNewHistory))
					for i := 0; i < len(test.expectNewHistory); i++ {
						assert.Equal(t, test.expectNewHistory[i].StartTimestamp.Time, backupRepo.Status.RecentMaintenance[i].StartTimestamp.Time)
						assert.Equal(t, test.expectNewHistory[i].CompleteTimestamp.Time, backupRepo.Status.RecentMaintenance[i].CompleteTimestamp.Time)
						assert.Equal(t, test.expectNewHistory[i].Result, backupRepo.Status.RecentMaintenance[i].Result)
						assert.Equal(t, test.expectNewHistory[i].Message, backupRepo.Status.RecentMaintenance[i].Message)
					}
				}

				if test.expectTimeUpdate != nil {
					assert.Equal(t, test.expectTimeUpdate.Time, backupRepo.Status.LastMaintenanceTime.Time)
				} else {
					assert.Equal(t, lastTm, backupRepo.Status.LastMaintenanceTime)
				}
			}
		})
	}
}

func TestConsolidateHistory(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		cur      []velerov1api.BackupRepositoryMaintenanceStatus
		coming   []velerov1api.BackupRepositoryMaintenanceStatus
		expected []velerov1api.BackupRepositoryMaintenanceStatus
	}{
		{
			name: "zero coming",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
			},
			expected: nil,
		},
		{
			name: "identical coming",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
			},
			coming: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
			},
			expected: nil,
		},
		{
			name: "less than limit",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
			},
			coming: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
			},
			expected: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
			},
		},
		{
			name: "more than limit",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
			},
			coming: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-4",
				},
			},
			expected: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-4",
				},
			},
		},
		{
			name: "more than limit 2",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
			},
			coming: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-4",
				},
			},
			expected: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-4",
				},
			},
		},
		{
			name: "coming is not used",
			cur: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-4",
				},
			},
			coming: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
			},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consolidated := consolidateHistory(test.coming, test.cur)
			if test.expected == nil {
				assert.Nil(t, consolidated)
			} else {
				assert.Len(t, consolidated, len(test.expected))
				for i := 0; i < len(test.expected); i++ {
					assert.Equal(t, *test.expected[i].StartTimestamp, *consolidated[i].StartTimestamp)
					assert.Equal(t, *test.expected[i].CompleteTimestamp, *consolidated[i].CompleteTimestamp)
					assert.Equal(t, test.expected[i].Result, consolidated[i].Result)
					assert.Equal(t, test.expected[i].Message, consolidated[i].Message)
				}

				assert.Nil(t, consolidateHistory(test.coming, consolidated))
			}
		})
	}
}

func TestGetLastMaintenanceTimeFromHistory(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		history  []velerov1api.BackupRepositoryMaintenanceStatus
		expected time.Time
	}{
		{
			name: "first one is nil",
			history: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp: &metav1.Time{Time: now},
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					Message:        "fake-maintenance-message",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
			},
			expected: now.Add(time.Hour * 3),
		},
		{
			name: "another one is nil",
			history: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					Message:        "fake-maintenance-message-2",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
			},
			expected: now.Add(time.Hour * 3),
		},
		{
			name: "disordered",
			history: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message-3",
				},
				{
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					Message:           "fake-maintenance-message",
				},
				{
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					Message:        "fake-maintenance-message-2",
				},
			},
			expected: now.Add(time.Hour * 3),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			time := getLastMaintenanceTimeFromHistory(test.history)
			assert.Equal(t, test.expected, time.Time)
		})
	}
}
