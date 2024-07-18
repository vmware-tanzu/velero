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
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, test.expected, resp)
		})
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name         string
		kubeconfig   string
		kubecontext  string
		QPS          float32
		burst        int
		expectedHost string
	}{
		{
			name:         "Test using the right cluster as context indexed",
			kubeconfig:   "kubeconfig",
			kubecontext:  "federal-context",
			QPS:          1.0,
			burst:        1,
			expectedHost: "https://horse.org:4443",
		},
		{
			name:         "Test using the right cluster as context indexed",
			kubeconfig:   "kubeconfig",
			kubecontext:  "queen-anne-context",
			QPS:          200.0,
			burst:        20,
			expectedHost: "https://pig.org:443",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := Config(test.kubeconfig, test.kubecontext, "velero", test.QPS, test.burst)
			require.NoError(t, err)
			assert.Equal(t, test.expectedHost, client.Host)
			assert.InDelta(t, test.QPS, client.QPS, 0.01)
			assert.Equal(t, test.burst, client.Burst)
		})
	}
}
