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

package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_parseResourceRequests(t *testing.T) {
	type args struct {
		cpuRequest string
		memRequest string
		cpuLimit   string
		memLimit   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"unbounded quantities", args{"0", "0", "0", "0"}, false},
		{"valid quantities", args{"100m", "128Mi", "200m", "256Mi"}, false},
		{"invalid quantity", args{"100m", "invalid", "200m", "256Mi"}, true},
		{"CPU request greater than limit", args{"300m", "128Mi", "200m", "256Mi"}, true},
		{"memory request greater than limit", args{"100m", "512Mi", "200m", "256Mi"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResourceRequests(tt.args.cpuRequest, tt.args.memRequest, tt.args.cpuLimit, tt.args.memLimit)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			expected := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(tt.args.cpuRequest),
					corev1.ResourceMemory: resource.MustParse(tt.args.memRequest),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(tt.args.cpuLimit),
					corev1.ResourceMemory: resource.MustParse(tt.args.memLimit),
				},
			}
			assert.Equal(t, expected, got)
		})
	}
}
