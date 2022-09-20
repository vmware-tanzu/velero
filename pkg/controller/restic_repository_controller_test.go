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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository"
	repomokes "github.com/vmware-tanzu/velero/pkg/repository/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

const testMaintenanceFrequency = 10 * time.Minute

func mockResticRepoReconciler(t *testing.T, rr *velerov1api.BackupRepository, mockOn string, arg interface{}, ret interface{}) *ResticRepoReconciler {
	mgr := &repomokes.Manager{}
	if mockOn != "" {
		mgr.On(mockOn, arg).Return(ret)
	}
	return NewResticRepoReconciler(
		velerov1api.DefaultNamespace,
		velerotest.NewLogger(),
		velerotest.NewFakeControllerRuntimeClient(t),
		testMaintenanceFrequency,
		mgr,
	)
}

func mockResticRepositoryCR() *velerov1api.BackupRepository {
	return &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Spec: velerov1api.BackupRepositorySpec{
			MaintenanceFrequency: metav1.Duration{testMaintenanceFrequency},
		},
	}

}

func TestPatchResticRepository(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "", nil, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	err = reconciler.patchResticRepository(context.Background(), rr, repoReady())
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.BackupRepositoryPhaseReady)
	err = reconciler.patchResticRepository(context.Background(), rr, repoNotReady("not ready"))
	assert.NoError(t, err)
	assert.NotEqual(t, rr.Status.Phase, velerov1api.BackupRepositoryPhaseReady)
}

func TestCheckNotReadyRepo(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "PrepareRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	err = reconciler.checkNotReadyRepo(context.TODO(), rr, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.BackupRepositoryPhase(""))
	rr.Spec.ResticIdentifier = "s3:test.amazonaws.com/bucket/restic"
	err = reconciler.checkNotReadyRepo(context.TODO(), rr, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.BackupRepositoryPhaseReady)
}

func TestRunMaintenanceIfDue(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "PruneRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	lastTm := rr.Status.LastMaintenanceTime
	err = reconciler.runMaintenanceIfDue(context.TODO(), rr, reconciler.logger)
	assert.NoError(t, err)
	assert.NotEqual(t, rr.Status.LastMaintenanceTime, lastTm)

	rr.Status.LastMaintenanceTime = &metav1.Time{Time: time.Now()}
	lastTm = rr.Status.LastMaintenanceTime
	err = reconciler.runMaintenanceIfDue(context.TODO(), rr, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.LastMaintenanceTime, lastTm)
}

func TestInitializeRepo(t *testing.T) {
	rr := mockResticRepositoryCR()
	rr.Spec.BackupStorageLocation = "default"
	reconciler := mockResticRepoReconciler(t, rr, "PrepareRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	locations := &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Config: map[string]string{"resticRepoPrefix": "s3:test.amazonaws.com/bucket/restic"},
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      rr.Spec.BackupStorageLocation,
		},
	}

	err = reconciler.Client.Create(context.TODO(), locations)
	assert.NoError(t, err)
	err = reconciler.initializeRepo(context.TODO(), rr, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.BackupRepositoryPhaseReady)
}

func TestResticRepoReconcile(t *testing.T) {
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
					MaintenanceFrequency: metav1.Duration{testMaintenanceFrequency},
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
					MaintenanceFrequency: metav1.Duration{testMaintenanceFrequency},
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
					MaintenanceFrequency: metav1.Duration{testMaintenanceFrequency},
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
			reconciler := mockResticRepoReconciler(t, test.repo, "", test.repo, nil)
			err := reconciler.Client.Create(context.TODO(), test.repo)
			assert.NoError(t, err)
			_, err = reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.repo.Namespace, Name: test.repo.Name}})
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
		mgr             repository.Manager
		repo            *velerov1api.BackupRepository
		freqReturn      time.Duration
		freqError       error
		userDefinedFreq time.Duration
		expectFreq      time.Duration
	}{
		{
			name:            "user defined valid",
			userDefinedFreq: time.Duration(time.Hour),
			expectFreq:      time.Duration(time.Hour),
		},
		{
			name:       "repo return valid",
			freqReturn: time.Duration(time.Hour * 2),
			expectFreq: time.Duration(time.Hour * 2),
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
			reconciler := NewResticRepoReconciler(
				velerov1api.DefaultNamespace,
				velerotest.NewLogger(),
				velerotest.NewFakeControllerRuntimeClient(t),
				test.userDefinedFreq,
				&mgr,
			)

			freq := reconciler.getRepositoryMaintenanceFrequency(test.repo)
			assert.Equal(t, test.expectFreq, freq)
		})
	}
}
