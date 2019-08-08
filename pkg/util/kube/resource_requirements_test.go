/*
Copyright 2019 the Velero contributors.

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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestParseResourceRequirements(t *testing.T) {
	type args struct {
		cpuRequest string
		memRequest string
		cpuLimit   string
		memLimit   string
	}
	tests := []struct {
		name     string
		args     args
		wantErr  bool
		expected *corev1.ResourceRequirements
	}{
		{"unbounded quantities", args{"0", "0", "0", "0"}, false, &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}},
		{"valid quantities", args{"100m", "128Mi", "200m", "256Mi"}, false, nil},
		{"CPU request with unbounded limit", args{"100m", "128Mi", "0", "256Mi"}, false, &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		}},
		{"Mem request with unbounded limit", args{"100m", "128Mi", "200m", "0"}, false, &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("200m"),
			},
		}},
		{"CPU/Mem requests with unbounded limits", args{"100m", "128Mi", "0", "0"}, false, &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{},
		}},
		{"invalid quantity", args{"100m", "invalid", "200m", "256Mi"}, true, nil},
		{"CPU request greater than limit", args{"300m", "128Mi", "200m", "256Mi"}, true, nil},
		{"memory request greater than limit", args{"100m", "512Mi", "200m", "256Mi"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourceRequirements(tt.args.cpuRequest, tt.args.memRequest, tt.args.cpuLimit, tt.args.memLimit)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			var expected corev1.ResourceRequirements
			if tt.expected == nil {
				expected = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(tt.args.cpuRequest),
						corev1.ResourceMemory: resource.MustParse(tt.args.memRequest),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(tt.args.cpuLimit),
						corev1.ResourceMemory: resource.MustParse(tt.args.memLimit),
					},
				}
			} else {
				expected = *tt.expected
			}

			assert.Equal(t, expected, got)
		})
	}
}
