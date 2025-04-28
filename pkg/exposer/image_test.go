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

package exposer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetInheritedPodInfoWithPriorityClassName(t *testing.T) {
	testCases := []struct {
		name              string
		priorityClassName string
		expectedValue     string
	}{
		{
			name:              "with priority class name",
			priorityClassName: "high-priority",
			expectedValue:     "high-priority",
		},
		{
			name:              "without priority class name",
			priorityClassName: "",
			expectedValue:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Create a deployment with the specified priority class name
			deployment := &appsv1api.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "velero",
					Namespace: "velero",
				},
				Spec: appsv1api.DeploymentSpec{
					Template: corev1api.PodTemplateSpec{
						Spec: corev1api.PodSpec{
							Containers: []corev1api.Container{
								{
									Name:  "velero",
									Image: "velero/velero:latest",
									Args:  []string{"server"},
								},
							},
							PriorityClassName: tc.priorityClassName,
						},
					},
				},
			}

			// Create a node-agent daemonset with the same priority class name
			daemonset := &appsv1api.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-agent",
					Namespace: "velero",
				},
				Spec: appsv1api.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "node-agent",
						},
					},
					Template: corev1api.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"name": "node-agent",
							},
						},
						Spec: corev1api.PodSpec{
							Containers: []corev1api.Container{
								{
									Name:  "node-agent",
									Image: "velero/velero:latest",
								},
							},
							PriorityClassName: tc.priorityClassName,
						},
					},
				},
			}

			// Add the deployment and daemonset to the fake client
			_, err := client.AppsV1().Deployments("velero").Create(context.Background(), deployment, metav1.CreateOptions{})
			require.NoError(t, err)

			_, err = client.AppsV1().DaemonSets("velero").Create(context.Background(), daemonset, metav1.CreateOptions{})
			require.NoError(t, err)

			// Call getInheritedPodInfo
			podInfo, err := getInheritedPodInfo(context.Background(), client, "velero", "linux")
			require.NoError(t, err)

			// Verify the priority class name is set correctly
			assert.Equal(t, tc.expectedValue, podInfo.priorityClassName)
		})
	}
}
