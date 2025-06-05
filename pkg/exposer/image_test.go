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

package exposer

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/util/kube"

	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetInheritedPodInfo(t *testing.T) {
	daemonSet := &appsv1api.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
	}

	daemonSetWithNoLog := &appsv1api.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
		Spec: appsv1api.DaemonSetSpec{
			Template: corev1api.PodTemplateSpec{
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name:  "container-1",
							Image: "image-1",
							Env: []corev1api.EnvVar{
								{
									Name:  "env-1",
									Value: "value-1",
								},
								{
									Name:  "env-2",
									Value: "value-2",
								},
							},
							EnvFrom: []corev1api.EnvFromSource{
								{
									ConfigMapRef: &corev1api.ConfigMapEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
											Name: "test-configmap",
										},
									},
								},
								{
									SecretRef: &corev1api.SecretEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
											Name: "test-secret",
										},
									},
								},
							},
							VolumeMounts: []corev1api.VolumeMount{
								{
									Name: "volume-1",
								},
								{
									Name: "volume-2",
								},
							},
						},
					},
					Volumes: []corev1api.Volume{
						{
							Name: "volume-1",
						},
						{
							Name: "volume-2",
						},
					},
					ServiceAccountName: "sa-1",
				},
			},
		},
	}

	daemonSetWithLog := &appsv1api.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
		Spec: appsv1api.DaemonSetSpec{
			Template: corev1api.PodTemplateSpec{
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name:  "container-1",
							Image: "image-1",
							Env: []corev1api.EnvVar{
								{
									Name:  "env-1",
									Value: "value-1",
								},
								{
									Name:  "env-2",
									Value: "value-2",
								},
							},
							EnvFrom: []corev1api.EnvFromSource{
								{
									ConfigMapRef: &corev1api.ConfigMapEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
											Name: "test-configmap",
										},
									},
								},
								{
									SecretRef: &corev1api.SecretEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
											Name: "test-secret",
										},
									},
								},
							},
							VolumeMounts: []corev1api.VolumeMount{
								{
									Name: "volume-1",
								},
								{
									Name: "volume-2",
								},
							},
							Args: []string{
								"--log-format=json",
								"--log-level",
								"debug",
							},
							Command: []string{
								"command-1",
							},
						},
					},
					Volumes: []corev1api.Volume{
						{
							Name: "volume-1",
						},
						{
							Name: "volume-2",
						},
					},
					ServiceAccountName: "sa-1",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	appsv1api.AddToScheme(scheme)

	tests := []struct {
		name          string
		namespace     string
		client        kubernetes.Interface
		kubeClientObj []runtime.Object
		result        inheritedPodInfo
		expectErr     string
	}{
		{
			name:      "ds is not found",
			namespace: "fake-ns",
			expectErr: "error to get node-agent pod template: error to get node-agent daemonset: daemonsets.apps \"node-agent\" not found",
		},
		{
			name:      "ds pod container number is invalidate",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectErr: "unexpected pod template from node-agent",
		},
		{
			name:      "no log info",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				daemonSetWithNoLog,
			},
			result: inheritedPodInfo{
				image:          "image-1",
				serviceAccount: "sa-1",
				env: []corev1api.EnvVar{
					{
						Name:  "env-1",
						Value: "value-1",
					},
					{
						Name:  "env-2",
						Value: "value-2",
					},
				},
				envFrom: []corev1api.EnvFromSource{
					{
						ConfigMapRef: &corev1api.ConfigMapEnvSource{
							LocalObjectReference: corev1api.LocalObjectReference{
								Name: "test-configmap",
							},
						},
					},
					{
						SecretRef: &corev1api.SecretEnvSource{
							LocalObjectReference: corev1api.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
				},
				volumeMounts: []corev1api.VolumeMount{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
				volumes: []corev1api.Volume{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
			},
		},
		{
			name:      "with log info",
			namespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				daemonSetWithLog,
			},
			result: inheritedPodInfo{
				image:          "image-1",
				serviceAccount: "sa-1",
				env: []corev1api.EnvVar{
					{
						Name:  "env-1",
						Value: "value-1",
					},
					{
						Name:  "env-2",
						Value: "value-2",
					},
				},
				envFrom: []corev1api.EnvFromSource{
					{
						ConfigMapRef: &corev1api.ConfigMapEnvSource{
							LocalObjectReference: corev1api.LocalObjectReference{
								Name: "test-configmap",
							},
						},
					},
					{
						SecretRef: &corev1api.SecretEnvSource{
							LocalObjectReference: corev1api.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
				},
				volumeMounts: []corev1api.VolumeMount{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
				volumes: []corev1api.Volume{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
				logFormatArgs: []string{
					"--log-format=json",
				},
				logLevelArgs: []string{
					"--log-level",
					"debug",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)
			info, err := getInheritedPodInfo(context.Background(), fakeKubeClient, test.namespace, kube.NodeOSLinux)

			if test.expectErr == "" {
				assert.NoError(t, err)
				assert.True(t, reflect.DeepEqual(info, test.result))
			} else {
				assert.EqualError(t, err, test.expectErr)
			}
		})
	}
}

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
