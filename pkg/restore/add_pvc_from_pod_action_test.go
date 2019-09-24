/*
Copyright 2019 the Velero contributors.

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

package restore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestAddPVCFromPodActionExecute(t *testing.T) {
	tests := []struct {
		name string
		item *v1.Pod
		want []velero.ResourceIdentifier
	}{
		{
			name: "pod with no volumes returns no additional items",
			item: &v1.Pod{},
			want: nil,
		},
		{
			name: "pod with some PVCs returns them as additional items",
			item: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "foo",
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							VolumeSource: v1.VolumeSource{
								EmptyDir: new(v1.EmptyDirVolumeSource),
							},
						},
						{
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-1",
								},
							},
						},
						{
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-2",
								},
							},
						},
						{
							VolumeSource: v1.VolumeSource{
								HostPath: new(v1.HostPathVolumeSource),
							},
						},
						{
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-3",
								},
							},
						},
					},
				},
			},
			want: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumeClaims, Namespace: "ns-1", Name: "pvc-1"},
				{GroupResource: kuberesource.PersistentVolumeClaims, Namespace: "ns-1", Name: "pvc-2"},
				{GroupResource: kuberesource.PersistentVolumeClaims, Namespace: "ns-1", Name: "pvc-3"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			itemData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.item)
			require.NoError(t, err)

			action := &AddPVCFromPodAction{logger: velerotest.NewLogger()}

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{Object: itemData},
			}

			res, err := action.Execute(input)
			require.NoError(t, err)

			assert.Equal(t, test.want, res.AdditionalItems)
		})
	}
}
