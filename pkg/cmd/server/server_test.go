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

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/controller"
	discovery_mocks "github.com/vmware-tanzu/velero/pkg/discovery/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

func TestVeleroResourcesExist(t *testing.T) {
	var (
		fakeDiscoveryHelper = &velerotest.FakeDiscoveryHelper{}
		server              = &server{
			logger:          velerotest.NewLogger(),
			discoveryHelper: fakeDiscoveryHelper,
		}
	)

	// Velero API group doesn't exist in discovery: should error
	fakeDiscoveryHelper.ResourceList = []*metav1.APIResourceList{
		{
			GroupVersion: "foo/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "Backups",
					Kind: "Backup",
				},
			},
		},
	}
	assert.Error(t, server.veleroResourcesExist())

	// Velero v1 API group doesn't contain any custom resources: should error
	veleroAPIResourceListVelerov1 := &metav1.APIResourceList{
		GroupVersion: velerov1api.SchemeGroupVersion.String(),
	}

	fakeDiscoveryHelper.ResourceList = append(fakeDiscoveryHelper.ResourceList, veleroAPIResourceListVelerov1)
	assert.Error(t, server.veleroResourcesExist())

	// Velero v2alpha1 API group doesn't contain any custom resources: should error
	veleroAPIResourceListVeleroV2alpha1 := &metav1.APIResourceList{
		GroupVersion: velerov2alpha1api.SchemeGroupVersion.String(),
	}

	fakeDiscoveryHelper.ResourceList = append(fakeDiscoveryHelper.ResourceList, veleroAPIResourceListVeleroV2alpha1)
	assert.Error(t, server.veleroResourcesExist())

	// Velero v1 API group contains all custom resources, but v2alpha1 doesn't contain any custom resources: should error
	for kind := range velerov1api.CustomResources() {
		veleroAPIResourceListVelerov1.APIResources = append(veleroAPIResourceListVelerov1.APIResources, metav1.APIResource{
			Kind: kind,
		})
	}
	assert.Error(t, server.veleroResourcesExist())

	// Velero v1 and v2alpha1 API group contain all custom resources: should not error
	for kind := range velerov2alpha1api.CustomResources() {
		veleroAPIResourceListVeleroV2alpha1.APIResources = append(veleroAPIResourceListVeleroV2alpha1.APIResources, metav1.APIResource{
			Kind: kind,
		})
	}
	assert.NoError(t, server.veleroResourcesExist())

	// Velero API group contains some but not all custom resources: should error
	veleroAPIResourceListVelerov1.APIResources = veleroAPIResourceListVelerov1.APIResources[:3]
	assert.Error(t, server.veleroResourcesExist())
}

func TestRemoveControllers(t *testing.T) {
	logger := velerotest.NewLogger()

	tests := []struct {
		name                string
		disabledControllers []string
		errorExpected       bool
	}{
		{
			name: "Remove one disable controller",
			disabledControllers: []string{
				controller.Backup,
			},
			errorExpected: false,
		},
		{
			name: "Remove all disable controllers",
			disabledControllers: []string{
				controller.BackupOperations,
				controller.Backup,
				controller.BackupDeletion,
				controller.BackupSync,
				controller.DownloadRequest,
				controller.GarbageCollection,
				controller.BackupRepo,
				controller.Restore,
				controller.Schedule,
				controller.ServerStatusRequest,
			},
			errorExpected: false,
		},
		{
			name: "Remove with a non-disable controller included",
			disabledControllers: []string{
				controller.Backup,
				controller.BackupStorageLocation,
			},
			errorExpected: true,
		},
		{
			name: "Remove with a misspelled/non-existing controller name",
			disabledControllers: []string{
				"go",
			},
			errorExpected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabledRuntimeControllers := map[string]struct{}{
				controller.BackupSync:          {},
				controller.Backup:              {},
				controller.GarbageCollection:   {},
				controller.Restore:             {},
				controller.ServerStatusRequest: {},
				controller.Schedule:            {},
				controller.BackupDeletion:      {},
				controller.BackupRepo:          {},
				controller.DownloadRequest:     {},
				controller.BackupOperations:    {},
			}

			totalNumOriginalControllers := len(enabledRuntimeControllers)

			if tt.errorExpected {
				assert.Error(t, removeControllers(tt.disabledControllers, enabledRuntimeControllers, logger))
			} else {
				assert.NoError(t, removeControllers(tt.disabledControllers, enabledRuntimeControllers, logger))

				totalNumEnabledControllers := len(enabledRuntimeControllers)
				assert.Equal(t, totalNumEnabledControllers, totalNumOriginalControllers-len(tt.disabledControllers))

				for _, disabled := range tt.disabledControllers {
					_, ok := enabledRuntimeControllers[disabled]
					assert.False(t, ok)
				}
			}
		})
	}
}

func TestNewCommand(t *testing.T) {
	assert.NotNil(t, NewCommand(nil))
}

func Test_newServer(t *testing.T) {
	factory := &mocks.Factory{}
	logger := logrus.New()

	// invalid uploader type
	_, err := newServer(factory, serverConfig{
		uploaderType: "invalid",
	}, logger)
	assert.NotNil(t, err)

	// invalid clientQPS
	_, err = newServer(factory, serverConfig{
		uploaderType: uploader.KopiaType,
		clientQPS:    -1,
	}, logger)
	assert.NotNil(t, err)

	// invalid clientBurst
	factory.On("SetClientQPS", mock.Anything).Return()
	_, err = newServer(factory, serverConfig{
		uploaderType: uploader.KopiaType,
		clientQPS:    1,
		clientBurst:  -1,
	}, logger)
	assert.NotNil(t, err)

	// invalid clientBclientPageSizeurst
	factory.On("SetClientQPS", mock.Anything).Return().
		On("SetClientBurst", mock.Anything).Return()
	_, err = newServer(factory, serverConfig{
		uploaderType:   uploader.KopiaType,
		clientQPS:      1,
		clientBurst:    1,
		clientPageSize: -1,
	}, logger)
	assert.NotNil(t, err)

	// got error when creating client
	factory.On("SetClientQPS", mock.Anything).Return().
		On("SetClientBurst", mock.Anything).Return().
		On("KubeClient").Return(nil, nil).
		On("Client").Return(nil, nil).
		On("DynamicClient").Return(nil, errors.New("error"))
	_, err = newServer(factory, serverConfig{
		uploaderType:   uploader.KopiaType,
		clientQPS:      1,
		clientBurst:    1,
		clientPageSize: 100,
	}, logger)
	assert.NotNil(t, err)
}

func Test_namespaceExists(t *testing.T) {
	client := kubefake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "velero",
		},
	})
	server := &server{
		kubeClient: client,
		logger:     logrus.New(),
	}

	// namespace doesn't exist
	assert.NotNil(t, server.namespaceExists("not-exist"))

	// namespace exists
	assert.Nil(t, server.namespaceExists("velero"))
}

func Test_veleroResourcesExist(t *testing.T) {
	helper := &discovery_mocks.Helper{}
	server := &server{
		discoveryHelper: helper,
		logger:          logrus.New(),
	}

	// velero resources don't exist
	helper.On("Resources").Return(nil)
	assert.NotNil(t, server.veleroResourcesExist())

	// velero resources exist
	helper.On("Resources").Unset()
	helper.On("Resources").Return([]*metav1.APIResourceList{
		{
			GroupVersion: velerov1api.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Kind: "Backup"},
				{Kind: "Restore"},
				{Kind: "Schedule"},
				{Kind: "DownloadRequest"},
				{Kind: "DeleteBackupRequest"},
				{Kind: "PodVolumeBackup"},
				{Kind: "PodVolumeRestore"},
				{Kind: "BackupRepository"},
				{Kind: "BackupStorageLocation"},
				{Kind: "VolumeSnapshotLocation"},
				{Kind: "ServerStatusRequest"},
			},
		},
		{
			GroupVersion: velerov2alpha1api.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Kind: "DataUpload"},
				{Kind: "DataDownload"},
			},
		},
	})
	assert.Nil(t, server.veleroResourcesExist())
}

func Test_markInProgressBackupsFailed(t *testing.T) {
	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&velerov1api.BackupList{
			Items: []velerov1api.Backup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "velero",
						Name:      "backup01",
					},
					Status: velerov1api.BackupStatus{
						Phase: velerov1api.BackupPhaseInProgress,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "velero",
						Name:      "backup02",
					},
					Status: velerov1api.BackupStatus{
						Phase: velerov1api.BackupPhaseCompleted,
					},
				},
			},
		}).
		Build()
	markInProgressBackupsFailed(context.Background(), c, "velero", logrus.New())

	backup01 := &velerov1api.Backup{}
	require.Nil(t, c.Get(context.Background(), client.ObjectKey{Namespace: "velero", Name: "backup01"}, backup01))
	assert.Equal(t, velerov1api.BackupPhaseFailed, backup01.Status.Phase)

	backup02 := &velerov1api.Backup{}
	require.Nil(t, c.Get(context.Background(), client.ObjectKey{Namespace: "velero", Name: "backup02"}, backup02))
	assert.Equal(t, velerov1api.BackupPhaseCompleted, backup02.Status.Phase)
}

func Test_markInProgressRestoresFailed(t *testing.T) {
	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&velerov1api.RestoreList{
			Items: []velerov1api.Restore{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "velero",
						Name:      "restore01",
					},
					Status: velerov1api.RestoreStatus{
						Phase: velerov1api.RestorePhaseInProgress,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "velero",
						Name:      "restore02",
					},
					Status: velerov1api.RestoreStatus{
						Phase: velerov1api.RestorePhaseCompleted,
					},
				},
			},
		}).
		Build()
	markInProgressRestoresFailed(context.Background(), c, "velero", logrus.New())

	restore01 := &velerov1api.Restore{}
	require.Nil(t, c.Get(context.Background(), client.ObjectKey{Namespace: "velero", Name: "restore01"}, restore01))
	assert.Equal(t, velerov1api.RestorePhaseFailed, restore01.Status.Phase)

	restore02 := &velerov1api.Restore{}
	require.Nil(t, c.Get(context.Background(), client.ObjectKey{Namespace: "velero", Name: "restore02"}, restore02))
	assert.Equal(t, velerov1api.RestorePhaseCompleted, restore02.Status.Phase)
}
