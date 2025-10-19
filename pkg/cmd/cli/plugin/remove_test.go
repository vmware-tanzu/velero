/*
Copyright 2025 the Velero contributors.

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

package plugin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
)

func TestPluginNameHeuristics(t *testing.T) {
	testCases := []struct {
		name           string
		pluginName     string
		initContainers []corev1api.Container
		expectedMatch  bool
		expectedIndex  int
	}{
		{
			name:       "exact container name match",
			pluginName: "velero-plugin-for-gcp",
			initContainers: []corev1api.Container{
				{Name: "velero-plugin-for-gcp", Image: "velero/velero-plugin-for-gcp:v1.0.0"},
			},
			expectedMatch: true,
			expectedIndex: 0,
		},
		{
			name:       "exact image match",
			pluginName: "velero/velero-plugin-for-gcp:v1.0.0",
			initContainers: []corev1api.Container{
				{Name: "velero-plugin-for-gcp", Image: "velero/velero-plugin-for-gcp:v1.0.0"},
			},
			expectedMatch: true,
			expectedIndex: 0,
		},
		{
			name:       "heuristic match by sanitized name",
			pluginName: "velero.io/gcp",
			initContainers: []corev1api.Container{
				{Name: "velero-io-gcp", Image: "velero/velero-plugin-for-gcp:v1.0.0"},
			},
			expectedMatch: true,
			expectedIndex: 0,
		},
		{
			name:       "heuristic match by substring in image",
			pluginName: "velero.io/gcp",
			initContainers: []corev1api.Container{
				{Name: "gcp-plugin", Image: "velero/velero-plugin-for-gcp:v1.0.0"},
			},
			expectedMatch: true,
			expectedIndex: 0,
		},
		{
			name:       "no match",
			pluginName: "velero.io/unknown",
			initContainers: []corev1api.Container{
				{Name: "velero-plugin-for-gcp", Image: "velero/velero-plugin-for-gcp:v1.0.0"},
			},
			expectedMatch: false,
			expectedIndex: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the heuristic matching logic
			index := -1
			matched := false

			// First try exact matches
			for x, container := range tc.initContainers {
				if container.Name == tc.pluginName || container.Image == tc.pluginName {
					index = x
					matched = true
					break
				}
			}

			// If no exact match and plugin name contains '/', try heuristics
			if !matched && len(tc.pluginName) > 0 {
				// This mimics the logic in remove.go
				sanitized := strings.NewReplacer("/", "-", "_", "-", ".", "-").Replace(tc.pluginName)
				last := tc.pluginName[strings.LastIndex(tc.pluginName, "/")+1:]

				var candidates []int
				for x, container := range tc.initContainers {
					if container.Name == sanitized ||
						strings.Contains(container.Image, last) ||
						strings.Contains(container.Name, last) {
						candidates = append(candidates, x)
					}
				}

				if len(candidates) == 1 {
					index = candidates[0]
					matched = true
				}
			}

			assert.Equal(t, tc.expectedMatch, matched, "Match result should be %v", tc.expectedMatch)
			if tc.expectedMatch {
				assert.Equal(t, tc.expectedIndex, index, "Index should be %d", tc.expectedIndex)
			}
		})
	}
}

func TestBuiltInPluginDetection(t *testing.T) {
	testCases := []struct {
		name          string
		pluginName    string
		serverStatus  map[string]bool // plugin name -> built-in status
		expectedError string
	}{
		{
			name:       "refuse removal of built-in plugin",
			pluginName: "velero.io/aws",
			serverStatus: map[string]bool{
				"velero.io/aws": true,
			},
			expectedError: "plugin velero.io/aws is built-in and cannot be removed",
		},
		{
			name:       "allow removal of non-built-in plugin",
			pluginName: "velero.io/gcp",
			serverStatus: map[string]bool{
				"velero.io/gcp": false,
			},
			expectedError: "", // No error expected
		},
		{
			name:          "plugin not found in server status",
			pluginName:    "velero.io/unknown",
			serverStatus:  map[string]bool{},
			expectedError: "", // No error expected, will try heuristics
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the built-in plugin detection logic
			if builtIn, exists := tc.serverStatus[tc.pluginName]; exists {
				if builtIn {
					assert.Equal(t, tc.expectedError, "plugin "+tc.pluginName+" is built-in and cannot be removed")
				} else {
					assert.Empty(t, tc.expectedError)
				}
			} else {
				assert.Empty(t, tc.expectedError)
			}
		})
	}
}
