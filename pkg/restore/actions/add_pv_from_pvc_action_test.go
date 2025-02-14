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

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestAddPVFromPVCActionExecute(t *testing.T) {
	tests := []struct {
		name           string
		itemFromBackup *corev1api.PersistentVolumeClaim
		want           []velero.ResourceIdentifier
	}{
		{
			name: "bound PVC with volume name returns associated PV",
			itemFromBackup: &corev1api.PersistentVolumeClaim{
				Spec: corev1api.PersistentVolumeClaimSpec{
					VolumeName: "bound-pv",
				},
				Status: corev1api.PersistentVolumeClaimStatus{
					Phase: corev1api.ClaimBound,
				},
			},
			want: []velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.PersistentVolumes,
					Name:          "bound-pv",
				},
			},
		},
		{
			name: "unbound PVC with volume name does not return any additional items",
			itemFromBackup: &corev1api.PersistentVolumeClaim{
				Spec: corev1api.PersistentVolumeClaimSpec{
					VolumeName: "pending-pv",
				},
				Status: corev1api.PersistentVolumeClaimStatus{
					Phase: corev1api.ClaimPending,
				},
			},
			want: nil,
		},
		{
			name: "bound PVC without volume name does not return any additional items",
			itemFromBackup: &corev1api.PersistentVolumeClaim{
				Spec: corev1api.PersistentVolumeClaimSpec{},
				Status: corev1api.PersistentVolumeClaimStatus{
					Phase: corev1api.ClaimBound,
				},
			},
			want: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			itemFromBackupData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.itemFromBackup)
			require.NoError(t, err)

			itemData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.itemFromBackup)
			require.NoError(t, err)
			// item should have no status
			delete(itemData, "status")

			action := &AddPVFromPVCAction{logger: velerotest.NewLogger()}

			input := &velero.RestoreItemActionExecuteInput{
				Item:           &unstructured.Unstructured{Object: itemData},
				ItemFromBackup: &unstructured.Unstructured{Object: itemFromBackupData},
			}

			res, err := action.Execute(input)
			require.NoError(t, err)

			assert.Equal(t, test.want, res.AdditionalItems)
		})
	}
}
