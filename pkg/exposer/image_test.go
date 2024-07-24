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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetInheritedPodInfo(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
	}

	daemonSetWithNoLog := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "container-1",
							Image: "image-1",
							Env: []v1.EnvVar{
								{
									Name:  "env-1",
									Value: "value-1",
								},
								{
									Name:  "env-2",
									Value: "value-2",
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name: "volume-1",
								},
								{
									Name: "volume-2",
								},
							},
						},
					},
					Volumes: []v1.Volume{
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

	daemonSetWithLog := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "DaemonSet",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "container-1",
							Image: "image-1",
							Env: []v1.EnvVar{
								{
									Name:  "env-1",
									Value: "value-1",
								},
								{
									Name:  "env-2",
									Value: "value-2",
								},
							},
							VolumeMounts: []v1.VolumeMount{
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
					Volumes: []v1.Volume{
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
	appsv1.AddToScheme(scheme)

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
				env: []v1.EnvVar{
					{
						Name:  "env-1",
						Value: "value-1",
					},
					{
						Name:  "env-2",
						Value: "value-2",
					},
				},
				volumeMounts: []v1.VolumeMount{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
				volumes: []v1.Volume{
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
				env: []v1.EnvVar{
					{
						Name:  "env-1",
						Value: "value-1",
					},
					{
						Name:  "env-2",
						Value: "value-2",
					},
				},
				volumeMounts: []v1.VolumeMount{
					{
						Name: "volume-1",
					},
					{
						Name: "volume-2",
					},
				},
				volumes: []v1.Volume{
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
			info, err := getInheritedPodInfo(context.Background(), fakeKubeClient, test.namespace)

			if test.expectErr == "" {
				assert.NoError(t, err)
				assert.True(t, reflect.DeepEqual(info, test.result))
			} else {
				assert.EqualError(t, err, test.expectErr)
			}
		})
	}
}
