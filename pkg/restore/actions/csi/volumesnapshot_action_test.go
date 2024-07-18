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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

var (
	testPVC       = "test-pvc"
	testSnapClass = "snap-class"
	randText      = "DEADFEED"
)

func TestResetVolumeSnapshotSpecForRestore(t *testing.T) {
	testCases := []struct {
		name    string
		vs      snapshotv1api.VolumeSnapshot
		vscName string
	}{
		{
			name: "should reset spec as expected",
			vs: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-ns",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					Source: snapshotv1api.VolumeSnapshotSource{
						PersistentVolumeClaimName: &testPVC,
					},
					VolumeSnapshotClassName: &testSnapClass,
				},
			},
			vscName: "test-vsc",
		},
		{
			name: "should reset spec and overwriting value for Source.VolumeSnapshotContentName",
			vs: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-ns",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					Source: snapshotv1api.VolumeSnapshotSource{
						VolumeSnapshotContentName: &randText,
					},
					VolumeSnapshotClassName: &testSnapClass,
				},
			},
			vscName: "test-vsc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.vs.DeepCopy()
			resetVolumeSnapshotSpecForRestore(&tc.vs, &tc.vscName)

			assert.Equalf(t, tc.vs.Name, before.Name, "unexpected change to Object.Name, Want: %s; Got %s", before.Name, tc.vs.Name)
			assert.Equalf(t, tc.vs.Namespace, before.Namespace, "unexpected change to Object.Namespace, Want: %s; Got %s", before.Namespace, tc.vs.Namespace)
			assert.NotNil(t, tc.vs.Spec.Source)
			assert.Nil(t, tc.vs.Spec.Source.PersistentVolumeClaimName)
			assert.NotNil(t, tc.vs.Spec.Source.VolumeSnapshotContentName)
			assert.Equal(t, *tc.vs.Spec.Source.VolumeSnapshotContentName, tc.vscName)
			assert.Equal(t, *tc.vs.Spec.VolumeSnapshotClassName, *before.Spec.VolumeSnapshotClassName, "unexpected value for Spec.VolumeSnapshotClassName, Want: %s, Got: %s",
				*tc.vs.Spec.VolumeSnapshotClassName, *before.Spec.VolumeSnapshotClassName)
			assert.Nil(t, tc.vs.Status)
		})
	}
}

func TestVSExecute(t *testing.T) {
	snapshotHandle := "vsc"
	tests := []struct {
		name        string
		item        runtime.Unstructured
		vs          *snapshotv1api.VolumeSnapshot
		restore     *velerov1api.Restore
		expectErr   bool
		createVS    bool
		expectedVSC *snapshotv1api.VolumeSnapshotContent
	}{
		{
			name:      "Restore's RestorePVs is false",
			restore:   builder.ForRestore("velero", "restore").RestorePVs(false).Result(),
			expectErr: false,
		},
		{
			name:      "Namespace remapping and VS already exists in cluster. Nothing change",
			vs:        builder.ForVolumeSnapshot("ns", "name").Result(),
			restore:   builder.ForRestore("velero", "restore").NamespaceMappings("ns", "newNS").Result(),
			createVS:  true,
			expectErr: false,
		},
		{
			name:      "VS doesn't have VolumeSnapshotHandleAnnotation annotation",
			vs:        builder.ForVolumeSnapshot("ns", "name").Result(),
			restore:   builder.ForRestore("velero", "restore").NamespaceMappings("ns", "newNS").Result(),
			expectErr: true,
		},
		{
			name: "VS doesn't have DriverNameAnnotation annotation",
			vs: builder.ForVolumeSnapshot("ns", "name").ObjectMeta(
				builder.WithAnnotations(velerov1api.VolumeSnapshotHandleAnnotation, ""),
			).Result(),
			restore:   builder.ForRestore("velero", "restore").NamespaceMappings("ns", "newNS").Result(),
			expectErr: true,
		},
		{
			name: "Normal case, VSC should be created",
			vs: builder.ForVolumeSnapshot("ns", "test").ObjectMeta(builder.WithAnnotationsMap(
				map[string]string{
					velerov1api.VolumeSnapshotHandleAnnotation: "vsc",
					velerov1api.DriverNameAnnotation:           "pd.csi.storage.gke.io",
				},
			)).Result(),
			restore:   builder.ForRestore("velero", "restore").Result(),
			expectErr: false,
			expectedVSC: builder.ForVolumeSnapshotContent("vsc").ObjectMeta(
				builder.WithLabels(velerov1api.RestoreNameLabel, "restore"),
			).VolumeSnapshotRef("ns", "test").
				DeletionPolicy(snapshotv1api.VolumeSnapshotContentRetain).
				Driver("pd.csi.storage.gke.io").
				Source(snapshotv1api.VolumeSnapshotContentSource{
					SnapshotHandle: &snapshotHandle,
				}).
				Result(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := volumeSnapshotRestoreItemAction{
				log:      logrus.StandardLogger(),
				crClient: velerotest.NewFakeControllerRuntimeClient(t),
			}

			if test.vs != nil {
				vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.vs)
				require.NoError(t, err)
				test.item = &unstructured.Unstructured{Object: vsMap}

				if test.createVS == true {
					if newNS, ok := test.restore.Spec.NamespaceMapping[test.vs.Namespace]; ok {
						test.vs.SetNamespace(newNS)
					}
					require.NoError(t, p.crClient.Create(context.TODO(), test.vs))
				}
			}

			_, err := p.Execute(
				&velero.RestoreItemActionExecuteInput{
					Item:    test.item,
					Restore: test.restore,
				},
			)

			if test.expectErr == false {
				require.NoError(t, err)
			}

			if test.expectedVSC != nil {
				vscList := new(snapshotv1api.VolumeSnapshotContentList)
				require.NoError(t, p.crClient.List(context.TODO(), vscList))
				require.True(t, cmp.Equal(
					*test.expectedVSC,
					vscList.Items[0],
					cmpopts.IgnoreFields(
						snapshotv1api.VolumeSnapshotContent{},
						"Kind", "APIVersion", "GenerateName", "Name",
						"ResourceVersion",
					),
				))
			}
		})
	}
}

func TestVSAppliesTo(t *testing.T) {
	p := volumeSnapshotRestoreItemAction{
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

func TestNewVolumeSnapshotRestoreItemAction(t *testing.T) {
	logger := logrus.StandardLogger()
	crClient := velerotest.NewFakeControllerRuntimeClient(t)

	f := &factorymocks.Factory{}
	f.On("KubebuilderClient").Return(nil, fmt.Errorf(""))
	plugin := NewVolumeSnapshotRestoreItemAction(f)
	_, err := plugin(logger)
	require.Error(t, err)

	f1 := &factorymocks.Factory{}
	f1.On("KubebuilderClient").Return(crClient, nil)
	plugin1 := NewVolumeSnapshotRestoreItemAction(f1)
	_, err1 := plugin1(logger)
	require.NoError(t, err1)
}
