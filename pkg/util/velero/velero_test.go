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

package velero

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPriorityClassNameFromVeleroServer(t *testing.T) {
	testCases := []struct {
		name                      string
		deployment                *appsv1api.Deployment
		expectedPriorityClassName string
	}{
		{
			name: "deployment with priority class name",
			deployment: &appsv1api.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "velero",
					Namespace: "velero",
				},
				Spec: appsv1api.DeploymentSpec{
					Template: corev1api.PodTemplateSpec{
						Spec: corev1api.PodSpec{
							PriorityClassName: "high-priority",
						},
					},
				},
			},
			expectedPriorityClassName: "high-priority",
		},
		{
			name: "deployment without priority class name",
			deployment: &appsv1api.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "velero",
					Namespace: "velero",
				},
				Spec: appsv1api.DeploymentSpec{
					Template: corev1api.PodTemplateSpec{
						Spec: corev1api.PodSpec{},
					},
				},
			},
			expectedPriorityClassName: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priorityClassName := GetPriorityClassNameFromVeleroServer(tc.deployment)
			assert.Equal(t, tc.expectedPriorityClassName, priorityClassName)
		})
	}
}
