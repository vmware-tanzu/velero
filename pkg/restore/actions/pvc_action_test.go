/*
Copyright 2020 the Velero contributors.

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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

// TestPVCActionExecute runs the PVCAction's Execute
// method and validates that the item's PVC is modified (or not) as expected.
// Validation is done by comparing the result of the Execute method to the test case's
// desired result.
func TestPVCActionExecute(t *testing.T) {
	tests := []struct {
		name    string
		pvc     *corev1api.PersistentVolumeClaim
		want    *corev1api.PersistentVolumeClaim
		wantErr error
	}{
		{
			name: "a persistent volume claim with no annotation",
			pvc:  builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
		},
		{
			name: "a persistent volume claim with selected-node annotation",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").ObjectMeta(builder.WithAnnotationsMap(map[string]string{})).Result(),
		},
		{
			name: "a persistent volume claim with other annotation",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("other-anno-1", "other-value-1", "other-anno-2", "other-value-2"),
				).Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").ObjectMeta(
				builder.WithAnnotations("other-anno-1", "other-value-1", "other-anno-2", "other-value-2"),
			).Result(),
		},
		{
			name: "a persistent volume claim with other annotation and selected-node annotation",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("other-anno", "other-value", "volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").ObjectMeta(
				builder.WithAnnotations("other-anno", "other-value"),
			).Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			a := NewPVCAction(
				velerotest.NewLogger(),
				clientset.CoreV1().ConfigMaps("velero"),
				clientset.CoreV1().Nodes(),
			)

			// set up test data
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pvc)
			require.NoError(t, err)

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: unstructuredMap,
				},
				ItemFromBackup: &unstructured.Unstructured{
					Object: unstructuredMap,
				},
			}

			// execute method under test
			res, err := a.Execute(input)

			// validate for both error and non-error cases
			switch {
			case tc.wantErr != nil:
				require.EqualError(t, err, tc.wantErr.Error())
			default:
				fmt.Printf("got +%v\n", res.UpdatedItem)
				require.NoError(t, err)

				wantUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.want)
				fmt.Printf("expected +%v\n", wantUnstructured)
				require.NoError(t, err)

				assert.Equal(t, &unstructured.Unstructured{Object: wantUnstructured}, res.UpdatedItem)
			}
		})
	}
}

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

			clientset := fake.NewSimpleClientset()
			action := NewPVCAction(
				velerotest.NewLogger(),
				clientset.CoreV1().ConfigMaps("velero"),
				clientset.CoreV1().Nodes(),
			)

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

func TestRemovePVCAnnotations(t *testing.T) {
	testCases := []struct {
		name                string
		pvc                 corev1api.PersistentVolumeClaim
		removeAnnotations   []string
		expectedAnnotations map[string]string
	}{
		{
			name: "should preserve all existing annotations",
			pvc: corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"ann1": "ann1-val",
						"ann2": "ann2-val",
						"ann3": "ann3-val",
						"ann4": "ann4-val",
					},
				},
			},
			removeAnnotations: []string{},
			expectedAnnotations: map[string]string{
				"ann1": "ann1-val",
				"ann2": "ann2-val",
				"ann3": "ann3-val",
				"ann4": "ann4-val",
			},
		},
		{
			name: "should remove all existing annotations",
			pvc: corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"ann1": "ann1-val",
						"ann2": "ann2-val",
						"ann3": "ann3-val",
						"ann4": "ann4-val",
					},
				},
			},
			removeAnnotations:   []string{"ann1", "ann2", "ann3", "ann4"},
			expectedAnnotations: map[string]string{},
		},
		{
			name: "should preserve some existing annotations",
			pvc: corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"ann1": "ann1-val",
						"ann2": "ann2-val",
						"ann3": "ann3-val",
						"ann4": "ann4-val",
						"ann5": "ann5-val",
						"ann6": "ann6-val",
						"ann7": "ann7-val",
						"ann8": "ann8-val",
					},
				},
			},
			removeAnnotations: []string{"ann1", "ann2", "ann3", "ann4"},
			expectedAnnotations: map[string]string{
				"ann5": "ann5-val",
				"ann6": "ann6-val",
				"ann7": "ann7-val",
				"ann8": "ann8-val",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			removePVCAnnotations(&tc.pvc, tc.removeAnnotations)
			assert.Equal(t, tc.expectedAnnotations, tc.pvc.Annotations)
		})
	}
}
