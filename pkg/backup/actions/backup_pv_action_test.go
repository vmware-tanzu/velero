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

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestBackupPVAction(t *testing.T) {
	pvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}

	backup := &v1.Backup{}

	a := NewPVCAction(velerotest.NewLogger())

	// no spec.volumeName should result in no error
	// and no additional items
	_, additional, err := a.Execute(pvc, backup)
	require.NoError(t, err)
	assert.Empty(t, additional)

	// empty spec.volumeName should result in no error
	// and no additional items
	pvc.Object["spec"].(map[string]interface{})["volumeName"] = ""
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	assert.Empty(t, additional)

	// Action should clean the spec.Selector when the StorageClassName is not set.
	input := builder.ForPersistentVolumeClaim("abc", "abc").VolumeName("pv").Selector(&metav1.LabelSelector{MatchLabels: map[string]string{"abc": "abc"}}).Phase(corev1.ClaimBound).Result()
	inputUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(input)
	require.NoError(t, err)
	item, additional, err := a.Execute(&unstructured.Unstructured{Object: inputUnstructured}, backup)
	require.NoError(t, err)
	require.Len(t, additional, 1)
	modifiedPVC := new(corev1.PersistentVolumeClaim)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), modifiedPVC))
	require.Nil(t, modifiedPVC.Spec.Selector)

	// Action should clean the spec.Selector when the StorageClassName is set to specific StorageClass
	input2 := builder.ForPersistentVolumeClaim("abc", "abc").VolumeName("pv").StorageClass("sc1").Selector(&metav1.LabelSelector{MatchLabels: map[string]string{"abc": "abc"}}).Phase(corev1.ClaimBound).Result()
	inputUnstructured2, err2 := runtime.DefaultUnstructuredConverter.ToUnstructured(input2)
	require.NoError(t, err2)
	item2, additional2, err2 := a.Execute(&unstructured.Unstructured{Object: inputUnstructured2}, backup)
	require.NoError(t, err2)
	require.Len(t, additional2, 1)
	modifiedPVC2 := new(corev1.PersistentVolumeClaim)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(item2.UnstructuredContent(), modifiedPVC2))
	require.Nil(t, modifiedPVC2.Spec.Selector)

	// Action should keep the spec.Selector when the StorageClassName is set to ""
	input3 := builder.ForPersistentVolumeClaim("abc", "abc").StorageClass("").Selector(&metav1.LabelSelector{MatchLabels: map[string]string{"abc": "abc"}}).VolumeName("pv").Phase(corev1.ClaimBound).Result()
	inputUnstructured3, err3 := runtime.DefaultUnstructuredConverter.ToUnstructured(input3)
	require.NoError(t, err3)
	item3, additional3, err3 := a.Execute(&unstructured.Unstructured{Object: inputUnstructured3}, backup)
	require.NoError(t, err3)
	require.Len(t, additional3, 1)
	modifiedPVC3 := new(corev1.PersistentVolumeClaim)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(item3.UnstructuredContent(), modifiedPVC3))
	require.Equal(t, input3.Spec.Selector, modifiedPVC3.Spec.Selector)

	// Action should delete label started with"velero.io/" from the spec.Selector when the StorageClassName is set to ""
	input4 := builder.ForPersistentVolumeClaim("abc", "abc").StorageClass("").Selector(&metav1.LabelSelector{MatchLabels: map[string]string{"velero.io/abc": "abc", "abc": "abc"}}).VolumeName("pv").Phase(corev1.ClaimBound).Result()
	inputUnstructured4, err4 := runtime.DefaultUnstructuredConverter.ToUnstructured(input4)
	require.NoError(t, err4)
	item4, additional4, err4 := a.Execute(&unstructured.Unstructured{Object: inputUnstructured4}, backup)
	require.NoError(t, err4)
	require.Len(t, additional4, 1)
	modifiedPVC4 := new(corev1.PersistentVolumeClaim)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(item4.UnstructuredContent(), modifiedPVC4))
	require.Equal(t, &metav1.LabelSelector{MatchLabels: map[string]string{"abc": "abc"}}, modifiedPVC4.Spec.Selector)

	// Action should clean the spec.Selector when the StorageClassName has value
	input5 := builder.ForPersistentVolumeClaim("abc", "abc").StorageClass("sc1").Selector(&metav1.LabelSelector{MatchLabels: map[string]string{"velero.io/abc": "abc", "abc": "abc"}}).VolumeName("pv").Phase(corev1.ClaimBound).Result()
	inputUnstructured5, err5 := runtime.DefaultUnstructuredConverter.ToUnstructured(input5)
	require.NoError(t, err5)
	item5, additional5, err5 := a.Execute(&unstructured.Unstructured{Object: inputUnstructured5}, backup)
	require.NoError(t, err5)
	require.Len(t, additional5, 1)
	modifiedPVC5 := new(corev1.PersistentVolumeClaim)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(item5.UnstructuredContent(), modifiedPVC5))
	require.Nil(t, modifiedPVC5.Spec.Selector)

	// non-empty spec.volumeName when status.phase is empty
	// should result in no error and no additional items
	pvc.Object["spec"].(map[string]interface{})["volumeName"] = "myVolume"
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	require.Empty(t, additional)

	// non-empty spec.volumeName when status.phase is 'Pending'
	// should result in no error and no additional items
	pvc.Object["status"].(map[string]interface{})["phase"] = corev1api.ClaimPending
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	require.Empty(t, additional)

	// non-empty spec.volumeName when status.phase is 'Lost'
	// should result in no error and no additional items
	pvc.Object["status"].(map[string]interface{})["phase"] = corev1api.ClaimLost
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	require.Empty(t, additional)

	// non-empty spec.volumeName when status.phase is 'Bound'
	// should result in no error and one additional item for the PV
	pvc.Object["status"].(map[string]interface{})["phase"] = corev1api.ClaimBound
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	require.Len(t, additional, 1)
	assert.Equal(t, velero.ResourceIdentifier{GroupResource: kuberesource.PersistentVolumes, Name: "myVolume"}, additional[0])

	// empty spec.volumeName when status.phase is 'Bound' should
	// result in no error and no additional items
	pvc.Object["spec"].(map[string]interface{})["volumeName"] = ""
	_, additional, err = a.Execute(pvc, backup)
	require.NoError(t, err)
	assert.Empty(t, additional)
}
