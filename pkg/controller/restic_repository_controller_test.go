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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	resticmokes "github.com/vmware-tanzu/velero/pkg/restic/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

const defaultMaintenanceFrequency = 10 * time.Minute

func mockResticRepoReconciler(t *testing.T, rr *velerov1api.ResticRepository, mockOn string, arg interface{}, ret interface{}) *ResticRepoReconciler {
	mgr := &resticmokes.RepositoryManager{}
	if mockOn != "" {
		mgr.On(mockOn, arg).Return(ret)
	}
	return NewResticRepoConciler(
		velerov1api.DefaultNamespace,
		velerotest.NewLogger(),
		velerotest.NewFakeControllerRuntimeClient(t),
		defaultMaintenanceFrequency,
		mgr,
	)
}

func mockResticRepositoryCR() *velerov1api.ResticRepository {
	return &velerov1api.ResticRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1api.DefaultNamespace,
			Name:      "repo",
		},
		Spec: velerov1api.ResticRepositorySpec{
			MaintenanceFrequency: metav1.Duration{defaultMaintenanceFrequency},
		},
	}

}

func TestPatchResticRepository(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "", nil, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	patchHelper, err := patch.NewHelper(rr, reconciler.Client)
	assert.NoError(t, err)
	err = reconciler.patchResticRepository(context.Background(), rr, patchHelper, reconciler.logger, repoReady())
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.ResticRepositoryPhaseReady)
	err = reconciler.patchResticRepository(context.Background(), rr, patchHelper, reconciler.logger, repoNotReady("not ready"))
	assert.NoError(t, err)
	assert.NotEqual(t, rr.Status.Phase, velerov1api.ResticRepositoryPhaseReady)
}

func TestCheckNotReadyRepo(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "ConnectToRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	patchHelper, err := patch.NewHelper(rr, reconciler.Client)
	assert.NoError(t, err)
	err = reconciler.checkNotReadyRepo(context.TODO(), rr, patchHelper, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.ResticRepositoryPhase(""))
	rr.Spec.ResticIdentifier = "s3:test.amazonaws.com/bucket/restic"
	err = reconciler.checkNotReadyRepo(context.TODO(), rr, patchHelper, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.ResticRepositoryPhaseReady)
}

func TestRunMaintenanceIfDue(t *testing.T) {
	rr := mockResticRepositoryCR()
	reconciler := mockResticRepoReconciler(t, rr, "PruneRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	patchHelper, err := patch.NewHelper(rr, reconciler.Client)
	assert.NoError(t, err)
	lastTm := rr.Status.LastMaintenanceTime
	err = reconciler.runMaintenanceIfDue(context.TODO(), rr, patchHelper, reconciler.logger)
	assert.NoError(t, err)
	assert.NotEqual(t, rr.Status.LastMaintenanceTime, lastTm)

	rr.Status.LastMaintenanceTime = &metav1.Time{Time: time.Now()}
	lastTm = rr.Status.LastMaintenanceTime
	err = reconciler.runMaintenanceIfDue(context.TODO(), rr, patchHelper, reconciler.logger)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.LastMaintenanceTime, lastTm)
}

func TestInitializeRepo(t *testing.T) {
	rr := mockResticRepositoryCR()
	rr.Spec.BackupStorageLocation = "default"
	reconciler := mockResticRepoReconciler(t, rr, "ConnectToRepo", rr, nil)
	err := reconciler.Client.Create(context.TODO(), rr)
	assert.NoError(t, err)
	patchHelper, err := patch.NewHelper(rr, reconciler.Client)
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
	err = reconciler.initializeRepo(context.TODO(), rr, reconciler.logger, patchHelper)
	assert.NoError(t, err)
	assert.Equal(t, rr.Status.Phase, velerov1api.ResticRepositoryPhaseReady)
}

func TestResticRepoReconcile(t *testing.T) {
	tests := []struct {
		name      string
		repo      *velerov1api.ResticRepository
		expectNil bool
	}{
		{
			name: "test on api server not found",
			repo: &velerov1api.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "unknown",
				},
				Spec: velerov1api.ResticRepositorySpec{
					MaintenanceFrequency: metav1.Duration{defaultMaintenanceFrequency},
				},
			},
			expectNil: true,
		},
		{
			name: "test on initialize repo",
			repo: &velerov1api.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.ResticRepositorySpec{
					MaintenanceFrequency: metav1.Duration{defaultMaintenanceFrequency},
				},
			},
			expectNil: true,
		},
		{
			name: "test on repo with new phase",
			repo: &velerov1api.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "repo",
				},
				Spec: velerov1api.ResticRepositorySpec{
					MaintenanceFrequency: metav1.Duration{defaultMaintenanceFrequency},
				},
				Status: velerov1api.ResticRepositoryStatus{
					Phase: velerov1api.ResticRepositoryPhaseNew,
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
