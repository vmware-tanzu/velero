/*
Copyright The Velero Contributors.

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

package controller

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func TestFindVolumeRestoresForPodLegacy(t *testing.T) {
	pod := &corev1api.Pod{}
	pod.UID = "uid"

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(velerov1api.SchemeGroupVersion, &velerov1api.PodVolumeRestore{}, &velerov1api.PodVolumeRestoreList{})
	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	// no matching PVR
	reconciler := &PodVolumeRestoreReconcilerLegacy{
		Client: clientBuilder.Build(),
		logger: logrus.New(),
	}
	requests := reconciler.findVolumeRestoresForPod(t.Context(), pod)
	assert.Empty(t, requests)

	// contain one matching PVR
	reconciler.Client = clientBuilder.WithLists(&velerov1api.PodVolumeRestoreList{
		Items: []velerov1api.PodVolumeRestore{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr1",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: string(pod.GetUID()),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr2",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: "non-matching-uid",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr3",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: string(pod.GetUID()),
					},
				},
				Spec: velerov1api.PodVolumeRestoreSpec{
					UploaderType: "kopia",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr4",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: string(pod.GetUID()),
					},
				},
				Spec: velerov1api.PodVolumeRestoreSpec{
					UploaderType: "restic",
				},
			},
		},
	}).Build()
	requests = reconciler.findVolumeRestoresForPod(t.Context(), pod)
	assert.Len(t, requests, 1)
}
