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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/controller"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
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

	// Velero API group doesn't contain any custom resources: should error
	veleroAPIResourceList := &metav1.APIResourceList{
		GroupVersion: v1.SchemeGroupVersion.String(),
	}

	fakeDiscoveryHelper.ResourceList = append(fakeDiscoveryHelper.ResourceList, veleroAPIResourceList)
	assert.Error(t, server.veleroResourcesExist())

	// Velero API group contains all custom resources: should not error
	for kind := range v1.CustomResources() {
		veleroAPIResourceList.APIResources = append(veleroAPIResourceList.APIResources, metav1.APIResource{
			Kind: kind,
		})
	}
	assert.NoError(t, server.veleroResourcesExist())

	// Velero API group contains some but not all custom resources: should error
	veleroAPIResourceList.APIResources = veleroAPIResourceList.APIResources[:3]
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
