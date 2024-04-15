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

package csi

import (
	"context"
	"fmt"
	"testing"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestVSCExecute(t *testing.T) {
	snapshotHandleStr := "test"
	tests := []struct {
		name      string
		item      runtime.Unstructured
		vsc       *snapshotv1api.VolumeSnapshotContent
		backup    *velerov1api.Backup
		createVSC bool
		expectErr bool
	}{
		{
			name: "VolumeSnapshotContent doesn't have backup label",
			item: velerotest.UnstructuredOrDie(
				`
				{
					"apiVersion": "snapshot.storage.k8s.io/v1",
					"kind": "VolumeSnapshotContent",
					"metadata": {
						"namespace": "ns",
						"name": "foo"
					}
				}
				`,
			),
			backup:    builder.ForBackup("velero", "backup").Result(),
			expectErr: false,
		},
		{
			name:      "VolumeSnapshotContent doesn't exist in the cluster, no error",
			vsc:       builder.ForVolumeSnapshotContent("bar").ObjectMeta(builder.WithLabelsMap(map[string]string{velerov1api.BackupNameLabel: "backup"})).Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: &snapshotHandleStr}).Result(),
			backup:    builder.ForBackup("velero", "backup").Result(),
			expectErr: false,
		},
		{
			name:      "Normal case, VolumeSnapshot should be deleted",
			vsc:       builder.ForVolumeSnapshotContent("bar").ObjectMeta(builder.WithLabelsMap(map[string]string{velerov1api.BackupNameLabel: "backup"})).Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: &snapshotHandleStr}).Result(),
			backup:    builder.ForBackup("velero", "backup").Result(),
			expectErr: false,
			createVSC: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			logger := logrus.StandardLogger()

			p := volumeSnapshotContentDeleteItemAction{log: logger, crClient: crClient}

			if test.vsc != nil {
				vscMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.vsc)
				require.NoError(t, err)
				test.item = &unstructured.Unstructured{Object: vscMap}
			}

			if test.createVSC {
				require.NoError(t, crClient.Create(context.TODO(), test.vsc))
			}

			err := p.Execute(
				&velero.DeleteItemActionExecuteInput{
					Item:   test.item,
					Backup: test.backup,
				},
			)

			if test.expectErr == false {
				require.NoError(t, err)
			}
		})
	}
}

func TestVSCAppliesTo(t *testing.T) {
	p := volumeSnapshotContentDeleteItemAction{
		log: logrus.StandardLogger(),
	}
	selector, err := p.AppliesTo()

	require.NoError(t, err)

	require.Equal(
		t,
		velero.ResourceSelector{
			IncludedResources: []string{"volumesnapshotcontents.snapshot.storage.k8s.io"},
		},
		selector,
	)
}

func TestNewVolumeSnapshotContentDeleteItemAction(t *testing.T) {
	logger := logrus.StandardLogger()
	crClient := velerotest.NewFakeControllerRuntimeClient(t)

	f := &factorymocks.Factory{}
	f.On("KubebuilderClient").Return(nil, fmt.Errorf(""))
	plugin := NewVolumeSnapshotContentDeleteItemAction(f)
	_, err := plugin(logger)
	require.Error(t, err)

	f1 := &factorymocks.Factory{}
	f1.On("KubebuilderClient").Return(crClient, nil)
	plugin1 := NewVolumeSnapshotContentDeleteItemAction(f1)
	_, err1 := plugin1(logger)
	require.NoError(t, err1)
}
