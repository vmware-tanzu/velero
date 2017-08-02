/*
Copyright 2017 Heptio Inc.

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

package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expected  []*metav1.APIResourceList
	}{
		{
			name: "no resources",
		},
		{
			name: "no extensions, order is preserved",
			resources: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
			},
			expected: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
			},
		},
		{
			name: "extensions moves to end, order is preserved",
			resources: []*metav1.APIResourceList{
				{GroupVersion: "extensions/v1beta1"},
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
			},
			expected: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
				{GroupVersion: "extensions/v1beta1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("before")
			for _, r := range test.resources {
				t.Logf(r.GroupVersion)
			}
			sortResources(test.resources)
			t.Logf("after")
			for _, r := range test.resources {
				t.Logf(r.GroupVersion)
			}
			assert.Equal(t, test.expected, test.resources)
		})
	}
}
