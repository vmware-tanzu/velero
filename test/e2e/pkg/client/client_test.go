/*
Copyright 2018 the Velero contributors.

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
package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		os       string
		arch     string
		gitSha   string
		version  string
		expected string
	}{
		{
			name:     "Test general interpolation in correct order",
			command:  "velero",
			os:       "darwin",
			arch:     "amd64",
			gitSha:   "abc123",
			version:  "v0.1.1",
			expected: "velero/v0.1.1 (darwin/amd64) abc123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := buildUserAgent(test.command, test.version, test.gitSha, test.os, test.arch)
			assert.Equal(t, resp, test.expected)
		})
	}
}
