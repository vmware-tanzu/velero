/*
Copyright 2018, 2019 the Velero contributors.

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
package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image name with registry hostname and tag",
			image:    "gcr.io/my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with registry hostname, without tag",
			image:    "gcr.io/my-repo/my-image",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name without registry hostname, with tag",
			image:    "my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name without registry hostname, without tag",
			image:    "my-repo/my-image",
			expected: "my-repo-my-image",
		},
		{
			name:     "image name with registry hostname and port, and tag",
			image:    "mycustomregistry.io:8080/my-repo/my-image:latest",
			expected: "my-repo-my-image",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, getName(test.image))
		})
	}
}
