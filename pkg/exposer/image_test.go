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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/util/kube"
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
					ImagePullSecrets: []corev1api.LocalObjectReference{
						{
							Name: "imagePullSecret1",
						},
					},
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
				imagePullSecrets: []corev1api.LocalObjectReference{
					{
						Name: "imagePullSecret1",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)
			info, err := getInheritedPodInfo(t.Context(), fakeKubeClient, test.namespace, kube.NodeOSLinux)

			if test.expectErr == "" {
				require.NoError(t, err)
				assert.True(t, reflect.DeepEqual(info, test.result))
			} else {
				assert.EqualError(t, err, test.expectErr)
			}
		})
	}
}
