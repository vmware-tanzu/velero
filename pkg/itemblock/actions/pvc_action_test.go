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
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestBackupPVAction(t *testing.T) {
	tests := []struct {
		name            string
		pvc             *corev1api.PersistentVolumeClaim
		pods            []*corev1api.Pod
		expectedErr     error
		expectedRelated []velero.ResourceIdentifier
	}{
		{
			name:            "Test no volumeName",
			pvc:             builder.ForPersistentVolumeClaim("velero", "testPVC").Phase(corev1api.ClaimBound).Result(),
			expectedErr:     nil,
			expectedRelated: nil,
		},
		{
			name:            "Test empty volumeName",
			pvc:             builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("").Phase(corev1api.ClaimBound).Result(),
			expectedErr:     nil,
			expectedRelated: nil,
		},
		{
			name:            "Test no status phase",
			pvc:             builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Result(),
			expectedErr:     nil,
			expectedRelated: nil,
		},
		{
			name:            "Test pending status phase",
			pvc:             builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimPending).Result(),
			expectedErr:     nil,
			expectedRelated: nil,
		},
		{
			name:            "Test lost status phase",
			pvc:             builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimLost).Result(),
			expectedErr:     nil,
			expectedRelated: nil,
		},
		{
			name:        "Test with volume",
			pvc:         builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimBound).Result(),
			expectedErr: nil,
			expectedRelated: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumes, Name: "testPV"},
			},
		},
		{
			name: "Test with volume and one running pod",
			pvc:  builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimBound).Result(),
			pods: []*corev1api.Pod{
				builder.ForPod("velero", "testPod1").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodRunning).Result(),
			},
			expectedErr: nil,
			expectedRelated: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumes, Name: "testPV"},
				{GroupResource: kuberesource.Pods, Namespace: "velero", Name: "testPod1"},
			},
		},
		{
			name: "Test with volume and multiple running pods",
			pvc:  builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimBound).Result(),
			pods: []*corev1api.Pod{
				builder.ForPod("velero", "testPod1").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodRunning).Result(),
				builder.ForPod("velero", "testPod2").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodRunning).Result(),
				builder.ForPod("velero", "testPod3").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodRunning).Result(),
			},
			expectedErr: nil,
			expectedRelated: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumes, Name: "testPV"},
				{GroupResource: kuberesource.Pods, Namespace: "velero", Name: "testPod1"},
				{GroupResource: kuberesource.Pods, Namespace: "velero", Name: "testPod2"},
				{GroupResource: kuberesource.Pods, Namespace: "velero", Name: "testPod3"},
			},
		},
		{
			name: "Test with volume and multiple running pods, some not running",
			pvc:  builder.ForPersistentVolumeClaim("velero", "testPVC").VolumeName("testPV").Phase(corev1api.ClaimBound).Result(),
			pods: []*corev1api.Pod{
				builder.ForPod("velero", "testPod1").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodSucceeded).Result(),
				builder.ForPod("velero", "testPod2").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).NodeName("velero").Phase(corev1api.PodRunning).Result(),
				builder.ForPod("velero", "testPod3").Volumes(builder.ForVolume("testPVC").PersistentVolumeClaimSource("testPVC").Result()).Phase(corev1api.PodRunning).Result(),
			},
			expectedErr: nil,
			expectedRelated: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumes, Name: "testPV"},
				{GroupResource: kuberesource.Pods, Namespace: "velero", Name: "testPod2"},
			},
		},
	}

	backup := &v1.Backup{}
	logger := logrus.New()

	f := &factorymocks.Factory{}
	f.On("KubebuilderClient").Return(nil, fmt.Errorf(""))
	plugin := NewPVCAction(f)
	_, err := plugin(logger)
	require.Error(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			crClient := velerotest.NewFakeControllerRuntimeClient(t)
			f := &factorymocks.Factory{}
			f.On("KubebuilderClient").Return(crClient, nil)
			plugin := NewPVCAction(f)
			i, err := plugin(logger)
			require.NoError(t, err)
			a := i.(*PVCAction)

			if tc.pvc != nil {
				require.NoError(t, crClient.Create(context.Background(), tc.pvc))
			}
			for _, pod := range tc.pods {
				require.NoError(t, crClient.Create(context.Background(), pod))
			}

			pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tc.pvc)
			require.NoError(t, err)

			relatedItems, err := a.GetRelatedItems(&unstructured.Unstructured{Object: pvcMap}, backup)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedRelated, relatedItems)
		})
	}
}
