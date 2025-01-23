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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNodeSelectorFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   map[string]string
	}{
		{
			name: "no node selector",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{},
						},
					},
				},
			},
			want: map[string]string{},
		},
		{
			name: "node selector",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetNodeSelectorFromVeleroServer(test.deploy)
			if len(got) != len(test.want) {
				t.Errorf("expected node selector to have %d elements, got %d", len(test.want), len(got))
			}
			for k, v := range test.want {
				if got[k] != v {
					t.Errorf("expected node selector to have key %s with value %s, got %s", k, v, got[k])
				}
			}
		})
	}
}

func TestGetTolerationsFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   []v1.Toleration
	}{
		{
			name: "no tolerations",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Tolerations: []v1.Toleration{},
						},
					},
				},
			},
			want: []v1.Toleration{},
		},
		{
			name: "tolerations",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Tolerations: []v1.Toleration{
								{
									Key:      "foo",
									Operator: "Exists",
								},
							},
						},
					},
				},
			},
			want: []v1.Toleration{
				{
					Key:      "foo",
					Operator: "Exists",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetTolerationsFromVeleroServer(test.deploy)
			if len(got) != len(test.want) {
				t.Errorf("expected tolerations to have %d elements, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Errorf("expected toleration at index %d to be %v, got %v", i, want, got[i])
				}
			}
		})
	}
}

func TestGetAffinityFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   *v1.Affinity
	}{
		{
			name: "no affinity",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Affinity: nil,
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "affinity",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Affinity: &v1.Affinity{
								NodeAffinity: &v1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
										NodeSelectorTerms: []v1.NodeSelectorTerm{
											{
												MatchExpressions: []v1.NodeSelectorRequirement{
													{
														Key:      "foo",
														Operator: "In",
														Values:   []string{"bar"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: "In",
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetAffinityFromVeleroServer(test.deploy)

			if got == nil {
				if test.want != nil {
					t.Errorf("expected affinity to be %v, got nil", test.want)
				}
			} else {
				if test.want == nil {
					t.Errorf("expected affinity to be nil, got %v", got)
				} else {
					if got.NodeAffinity == nil {
						if test.want.NodeAffinity != nil {
							t.Errorf("expected node affinity to be %v, got nil", test.want.NodeAffinity)
						}
					} else {
						if test.want.NodeAffinity == nil {
							t.Errorf("expected node affinity to be nil, got %v", got.NodeAffinity)
						} else {
							if got.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
								if test.want.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
									t.Errorf("expected required during scheduling ignored during execution to be %v, got nil", test.want.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
								}
							} else {
								if test.want.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
									t.Errorf("expected required during scheduling ignored during execution to be nil, got %v", got.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
								} else {
									if !reflect.DeepEqual(got.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution, test.want.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution) {
										t.Errorf("expected required during scheduling ignored during execution to be %v, got %v", test.want.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution, got.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
									}
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestGetEnvVarsFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   []v1.EnvVar
	}{
		{
			name: "no env vars",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{},
								},
							},
						},
					},
				},
			},
			want: []v1.EnvVar{},
		},
		{
			name: "env vars",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Env: []v1.EnvVar{
										{
											Name:  "foo",
											Value: "bar",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []v1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetEnvVarsFromVeleroServer(test.deploy)
			if len(got) != len(test.want) {
				t.Errorf("expected env vars to have %d elements, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Errorf("expected env var at index %d to be %v, got %v", i, want, got[i])
				}
			}
		})
	}
}

func TestGetEnvFromSourcesFromVeleroServer(t *testing.T) {
	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected []v1.EnvFromSource
	}{
		{
			name: "no env vars",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									EnvFrom: []v1.EnvFromSource{},
								},
							},
						},
					},
				},
			},
			expected: []v1.EnvFromSource{},
		},
		{
			name: "configmap",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									EnvFrom: []v1.EnvFromSource{
										{
											ConfigMapRef: &v1.ConfigMapEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "foo",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []v1.EnvFromSource{
				{
					ConfigMapRef: &v1.ConfigMapEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
		},
		{
			name: "secret",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									EnvFrom: []v1.EnvFromSource{
										{
											SecretRef: &v1.SecretEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "foo",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []v1.EnvFromSource{
				{
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetEnvFromSourcesFromVeleroServer(test.deploy)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestGetVolumeMountsFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   []v1.VolumeMount
	}{
		{
			name: "no volume mounts",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									VolumeMounts: []v1.VolumeMount{},
								},
							},
						},
					},
				},
			},
			want: []v1.VolumeMount{},
		},
		{
			name: "volume mounts",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      "foo",
											MountPath: "/bar",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []v1.VolumeMount{
				{
					Name:      "foo",
					MountPath: "/bar",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetVolumeMountsFromVeleroServer(test.deploy)
			if len(got) != len(test.want) {
				t.Errorf("expected volume mounts to have %d elements, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Errorf("expected volume mount at index %d to be %v, got %v", i, want, got[i])
				}
			}
		})
	}
}

func TestGetVolumesFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   []v1.Volume
	}{
		{
			name: "no volumes",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{},
						},
					},
				},
			},
			want: []v1.Volume{},
		},
		{
			name: "volumes",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name: "foo",
								},
							},
						},
					},
				},
			},
			want: []v1.Volume{
				{
					Name: "foo",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetVolumesFromVeleroServer(test.deploy)
			if len(got) != len(test.want) {
				t.Errorf("expected volumes to have %d elements, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Errorf("expected volume at index %d to be %v, got %v", i, want, got[i])
				}
			}
		})
	}
}

func TestGetServiceAccountFromVeleroServer(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   string
	}{
		{
			name: "no service account",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							ServiceAccountName: "",
						},
					},
				},
			},
			want: "",
		},
		{
			name: "service account",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							ServiceAccountName: "foo",
						},
					},
				},
			},
			want: "foo",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetServiceAccountFromVeleroServer(test.deploy)
			if got != test.want {
				t.Errorf("expected service account to be %s, got %s", test.want, got)
			}
		})
	}
}

func TestGetVeleroServerImage(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   string
	}{
		{
			name: "velero server image",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Image: "velero/velero:latest",
								},
							},
						},
					},
				},
			},
			want: "velero/velero:latest",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetVeleroServerImage(test.deploy)
			if got != test.want {
				t.Errorf("expected velero server image to be %s, got %s", test.want, got)
			}
		})
	}
}

func TestGetVeleroServerLables(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		expected   map[string]string
	}{
		{
			name: "Empty Labels",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{},
						},
					},
				},
			},
			expected: map[string]string{},
		},
		{
			name: "Non-empty Labels",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":       "velero",
								"component": "server",
							},
						},
					},
				},
			},
			expected: map[string]string{
				"app":       "velero",
				"component": "server",
			},
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVeleroServerLables(tt.deployment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVeleroServerAnnotations(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		expected   map[string]string
	}{
		{
			name: "Empty Labels",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
					},
				},
			},
			expected: map[string]string{},
		},
		{
			name: "Non-empty Labels",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"app":       "velero",
								"component": "server",
							},
						},
					},
				},
			},
			expected: map[string]string{
				"app":       "velero",
				"component": "server",
			},
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVeleroServerAnnotations(tt.deployment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVeleroServerLabelValue(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		expected   string
	}{
		{
			name:       "nil Labels",
			deployment: &appsv1.Deployment{},
			expected:   "",
		},
		{
			name: "no label key",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{},
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "with label key",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"fake-key": "fake-value"},
						},
					},
				},
			},
			expected: "fake-value",
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVeleroServerLabelValue(tt.deployment, "fake-key")
			assert.Equal(t, tt.expected, result)
		})
	}
}
