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
	"testing"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

func TestVSCExecute(t *testing.T) {
	tests := []struct {
		name          string
		item          runtime.Unstructured
		vsc           *snapshotv1api.VolumeSnapshotContent
		restore       *velerov1api.Restore
		expectErr     bool
		expectedItems []velero.ResourceIdentifier
	}{
		{
			name:      "Restore's RestorePVs is false",
			restore:   builder.ForRestore("velero", "restore").RestorePVs(false).Result(),
			expectErr: false,
		},
		{
			name: "Normal case, additional items should return",
			vsc: builder.ForVolumeSnapshotContent("test").ObjectMeta(builder.WithAnnotationsMap(
				map[string]string{
					velerov1api.PrefixedSecretNameAnnotation:      "name",
					velerov1api.PrefixedSecretNamespaceAnnotation: "namespace",
				},
			)).Result(),
			restore:   builder.ForRestore("velero", "restore").Result(),
			expectErr: false,
			expectedItems: []velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "namespace",
					Name:          "name",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, err := NewVolumeSnapshotContentRestoreItemAction(logrus.StandardLogger())
			require.NoError(t, err)

			action := p.(*volumeSnapshotContentRestoreItemAction)

			if test.vsc != nil {
				vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.vsc)
				require.NoError(t, err)
				test.item = &unstructured.Unstructured{Object: vsMap}
			}

			output, err := action.Execute(
				&velero.RestoreItemActionExecuteInput{
					Item:    test.item,
					Restore: test.restore,
				},
			)

			if test.expectErr == false {
				require.NoError(t, err)
			}

			if len(test.expectedItems) > 0 {
				require.Equal(t, test.expectedItems, output.AdditionalItems)
			}
		})
	}
}

func TestVSCAppliesTo(t *testing.T) {
	p := volumeSnapshotContentRestoreItemAction{
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
