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

func TestVSExecute(t *testing.T) {
	tests := []struct {
		name      string
		item      runtime.Unstructured
		vs        *snapshotv1api.VolumeSnapshot
		backup    *velerov1api.Backup
		createVS  bool
		expectErr bool
	}{
		{
			name: "VolumeSnapshot doesn't have backup label",
			item: velerotest.UnstructuredOrDie(
				`
				{
					"apiVersion": "snapshot.storage.k8s.io/v1",
					"kind": "VolumeSnapshot",
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
			name: "VolumeSnapshot doesn't exist in the cluster",
			vs: builder.ForVolumeSnapshot("foo", "bar").
				ObjectMeta(builder.WithLabelsMap(
					map[string]string{velerov1api.BackupNameLabel: "backup"},
				)).Status().
				BoundVolumeSnapshotContentName("vsc").
				Result(),
			backup:    builder.ForBackup("velero", "backup").Result(),
			expectErr: true,
		},
		{
			name: "Normal case, VolumeSnapshot should be deleted",
			vs: builder.ForVolumeSnapshot("foo", "bar").
				ObjectMeta(builder.WithLabelsMap(
					map[string]string{velerov1api.BackupNameLabel: "backup"},
				)).Status().
				BoundVolumeSnapshotContentName("vsc").
				Result(),
			backup:    builder.ForBackup("velero", "backup").Result(),
			expectErr: false,
			createVS:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			logger := logrus.StandardLogger()

			p := volumeSnapshotDeleteItemAction{log: logger, crClient: crClient}

			if test.vs != nil {
				vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.vs)
				require.NoError(t, err)
				test.item = &unstructured.Unstructured{Object: vsMap}
			}

			if test.createVS {
				require.NoError(t, crClient.Create(context.TODO(), test.vs))
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

func TestVSAppliesTo(t *testing.T) {
	p := volumeSnapshotDeleteItemAction{
		log: logrus.StandardLogger(),
	}
	selector, err := p.AppliesTo()

	require.NoError(t, err)

	require.Equal(
		t,
		velero.ResourceSelector{
			IncludedResources: []string{"volumesnapshots.snapshot.storage.k8s.io"},
		},
		selector,
	)
}

func TestNewVolumeSnapshotDeleteItemAction(t *testing.T) {
	logger := logrus.StandardLogger()
	crClient := velerotest.NewFakeControllerRuntimeClient(t)

	f := &factorymocks.Factory{}
	f.On("KubebuilderClient").Return(nil, fmt.Errorf(""))
	plugin := NewVolumeSnapshotDeleteItemAction(f)
	_, err := plugin(logger)
	require.Error(t, err)

	f1 := &factorymocks.Factory{}
	f1.On("KubebuilderClient").Return(crClient, nil)
	plugin1 := NewVolumeSnapshotDeleteItemAction(f1)
	_, err1 := plugin1(logger)
	require.NoError(t, err1)
}
