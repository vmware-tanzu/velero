/*
Copyright the Velero contributors.

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

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestEnsureRepo(t *testing.T) {
	bkRepoObjReady := NewBackupRepository(velerov1.DefaultNamespace, BackupRepositoryKey{
		VolumeNamespace: "fake-ns",
		BackupLocation:  "fake-bsl",
		RepositoryType:  "fake-repo-type",
	})

	bkRepoObjReady.Status.Phase = velerov1.BackupRepositoryPhaseReady

	bkRepoObjNotReady := NewBackupRepository(velerov1.DefaultNamespace, BackupRepositoryKey{
		VolumeNamespace: "fake-ns",
		BackupLocation:  "fake-bsl",
		RepositoryType:  "fake-repo-type",
	})

	scheme := runtime.NewScheme()
	velerov1.AddToScheme(scheme)

	tests := []struct {
		name           string
		namespace      string
		bsl            string
		repositoryType string
		kubeClientObj  []runtime.Object
		runtimeScheme  *runtime.Scheme
		expectedRepo   *velerov1.BackupRepository
		err            string
	}{
		{
			name:           "namespace is empty",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			err:            "wrong parameters, namespace \"\", backup storage location \"fake-bsl\", repository type \"fake-repo-type\"",
		},
		{
			name:           "bsl is empty",
			namespace:      "fake-ns",
			repositoryType: "fake-repo-type",
			err:            "wrong parameters, namespace \"fake-ns\", backup storage location \"\", repository type \"fake-repo-type\"",
		},
		{
			name:      "repositoryType is empty",
			namespace: "fake-ns",
			bsl:       "fake-bsl",
			err:       "wrong parameters, namespace \"fake-ns\", backup storage location \"fake-bsl\", repository type \"\"",
		},
		{
			name:           "get repo fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			err:            "error getting backup repository list: no kind is registered for the type v1.BackupRepositoryList in scheme \"pkg/runtime/scheme.go:100\"",
		},
		{
			name:           "success on existing repo",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			kubeClientObj: []runtime.Object{
				bkRepoObjReady,
			},
			runtimeScheme: scheme,
			expectedRepo:  bkRepoObjReady,
		},
		{
			name:           "wait existing repo fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			kubeClientObj: []runtime.Object{
				bkRepoObjNotReady,
			},
			runtimeScheme: scheme,
			err:           "failed to wait BackupRepository, timeout exceeded: backup repository not provisioned",
		},
		{
			name:           "create fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			runtimeScheme:  scheme,
			err:            "failed to wait BackupRepository, timeout exceeded: backup repository not provisioned",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder()
			if test.runtimeScheme != nil {
				fakeClientBuilder = fakeClientBuilder.WithScheme(test.runtimeScheme)
			}

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			ensurer := NewEnsurer(fakeClient, velerotest.NewLogger(), time.Millisecond)

			repo, err := ensurer.EnsureRepo(context.Background(), velerov1.DefaultNamespace, test.namespace, test.bsl, test.repositoryType)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedRepo, repo)
		})
	}
}

func TestCreateBackupRepositoryAndWait(t *testing.T) {
	bkRepoObj := NewBackupRepository(velerov1.DefaultNamespace, BackupRepositoryKey{
		VolumeNamespace: "fake-ns",
		BackupLocation:  "fake-bsl",
		RepositoryType:  "fake-repo-type",
	})

	bkRepoObj.Status.Phase = velerov1.BackupRepositoryPhaseReady

	scheme := runtime.NewScheme()
	velerov1.AddToScheme(scheme)

	tests := []struct {
		name           string
		namespace      string
		bsl            string
		repositoryType string
		kubeClientObj  []runtime.Object
		runtimeScheme  *runtime.Scheme
		expectedRepo   *velerov1.BackupRepository
		err            string
	}{
		{
			name:           "create fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			err:            "unable to create backup repository resource: no kind is registered for the type v1.BackupRepository in scheme \"pkg/runtime/scheme.go:100\"",
		},
		{
			name:           "get repo fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			kubeClientObj: []runtime.Object{
				bkRepoObj,
			},
			runtimeScheme: scheme,
			err:           "failed to wait BackupRepository, errored early: more than one BackupRepository found for workload namespace \"fake-ns\", backup storage location \"fake-bsl\", repository type \"fake-repo-type\"",
		},
		{
			name:           "wait repo fail",
			namespace:      "fake-ns",
			bsl:            "fake-bsl",
			repositoryType: "fake-repo-type",
			runtimeScheme:  scheme,
			err:            "failed to wait BackupRepository, timeout exceeded: backup repository not provisioned",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder()
			if test.runtimeScheme != nil {
				fakeClientBuilder = fakeClientBuilder.WithScheme(test.runtimeScheme)
			}

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			ensurer := NewEnsurer(fakeClient, velerotest.NewLogger(), time.Millisecond)

			repo, err := ensurer.createBackupRepositoryAndWait(context.Background(), velerov1.DefaultNamespace, BackupRepositoryKey{
				VolumeNamespace: test.namespace,
				BackupLocation:  test.bsl,
				RepositoryType:  test.repositoryType,
			})
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedRepo, repo)
		})
	}
}
