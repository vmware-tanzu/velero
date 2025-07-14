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

package kibishii

import "testing"

func TestResolveBasePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/home/user/project/overlays/sc-reclaim-policy/azure",
			expected: "/home/user/project/",
		},
		{
			input:    "/go/src/github.com/org/repo/base",
			expected: "/go/src/github.com/org/repo/base",
		},
		{
			input:    "/some/dir/overlays",
			expected: "/some/dir/",
		},
		{
			input:    "overlays/sc-reclaim-policy/azure",
			expected: "",
		},
	}

	for _, tt := range tests {
		actual := resolveBasePath(tt.input)
		if actual != tt.expected {
			t.Errorf("resolveBasePath(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestStripRegistry(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "quay.io/example/image:tag",
			expected: "example/image:tag",
		},
		{
			input:    "docker.io/library/nginx:latest",
			expected: "library/nginx:latest",
		},
		{
			input:    "gcr.io/project/app",
			expected: "project/app",
		},
		{
			input:    "my-custom-reg.io/myapp",
			expected: "myapp",
		},
		{
			input:    "ubuntu:20.04",
			expected: "ubuntu:20.04",
		},
		{
			input:    "library/nginx",
			expected: "library/nginx",
		},
	}

	for _, tt := range tests {
		actual := stripRegistry(tt.input)
		if actual != tt.expected {
			t.Errorf("stripRegistry(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}
