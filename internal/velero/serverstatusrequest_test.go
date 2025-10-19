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

package velero

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
)

// MockPluginLister is a mock implementation of PluginLister interface
type MockPluginLister struct {
	mock.Mock
}

func (m *MockPluginLister) List(kind common.PluginKind) []framework.PluginIdentifier {
	args := m.Called(kind)
	return args.Get(0).([]framework.PluginIdentifier)
}

func TestGetInstalledPluginInfo(t *testing.T) {
	// Store original os.Args[0] to restore later
	originalArgs0 := os.Args[0]
	defer func() {
		os.Args[0] = originalArgs0
	}()

	testCases := []struct {
		name            string
		serverBinary    string
		plugins         map[common.PluginKind][]framework.PluginIdentifier
		expectedBuiltIn map[string]bool
	}{
		{
			name:         "built-in plugins marked correctly",
			serverBinary: "/usr/local/bin/velero",
			plugins: map[common.PluginKind][]framework.PluginIdentifier{
				common.PluginKindBackupItemAction: {
					{
						Name:    "velero.io/pod",
						Kind:    common.PluginKindBackupItemAction,
						Command: "/usr/local/bin/velero", // Same as server binary
					},
					{
						Name:    "velero.io/pv",
						Kind:    common.PluginKindBackupItemAction,
						Command: "/usr/local/bin/velero", // Same as server binary
					},
				},
				common.PluginKindObjectStore: {
					{
						Name:    "velero.io/aws",
						Kind:    common.PluginKindObjectStore,
						Command: "/usr/local/bin/velero", // Same as server binary
					},
				},
			},
			expectedBuiltIn: map[string]bool{
				"velero.io/pod": true,
				"velero.io/pv":  true,
				"velero.io/aws": true,
			},
		},
		{
			name:         "external plugins not marked as built-in",
			serverBinary: "/usr/local/bin/velero",
			plugins: map[common.PluginKind][]framework.PluginIdentifier{
				common.PluginKindBackupItemAction: {
					{
						Name:    "velero.io/pod",
						Kind:    common.PluginKindBackupItemAction,
						Command: "/usr/local/bin/velero", // Same as server binary
					},
				},
				common.PluginKindObjectStore: {
					{
						Name:    "velero.io/aws",
						Kind:    common.PluginKindObjectStore,
						Command: "/usr/local/bin/velero", // Same as server binary
					},
					{
						Name:    "velero.io/gcp",
						Kind:    common.PluginKindObjectStore,
						Command: "/plugins/gcp-plugin", // Different from server binary
					},
				},
			},
			expectedBuiltIn: map[string]bool{
				"velero.io/pod": true,
				"velero.io/aws": true,
				"velero.io/gcp": false,
			},
		},
		{
			name:            "empty plugin list",
			serverBinary:    "/usr/local/bin/velero",
			plugins:         map[common.PluginKind][]framework.PluginIdentifier{},
			expectedBuiltIn: map[string]bool{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up the server binary path
			os.Args[0] = tc.serverBinary

			// Create mock plugin lister
			mockLister := &MockPluginLister{}

			// Set up mock expectations for all plugin kinds
			for kind, plugins := range tc.plugins {
				mockLister.On("List", kind).Return(plugins)
			}

			// Set up mock expectations for any plugin kinds not in our test data
			allKinds := common.AllPluginKinds()
			for _, kind := range allKinds {
				if _, exists := tc.plugins[kind]; !exists {
					mockLister.On("List", kind).Return([]framework.PluginIdentifier{})
				}
			}

			// Call the function under test
			result := GetInstalledPluginInfo(mockLister)

			// Verify the results
			assert.Len(t, result, len(tc.expectedBuiltIn))

			// Create a map of results for easier verification
			resultMap := make(map[string]velerov1api.PluginInfo)
			for _, plugin := range result {
				resultMap[plugin.Name] = plugin
			}

			// Verify each expected plugin
			for pluginName, expectedBuiltIn := range tc.expectedBuiltIn {
				plugin, exists := resultMap[pluginName]
				assert.True(t, exists, "Plugin %s should exist in result", pluginName)
				assert.Equal(t, expectedBuiltIn, plugin.BuiltIn, "Plugin %s BuiltIn should be %v", pluginName, expectedBuiltIn)
				assert.NotEmpty(t, plugin.Command, "Plugin %s should have a Command", pluginName)
				assert.NotEmpty(t, plugin.Kind, "Plugin %s should have a Kind", pluginName)
			}

			// Verify all mock expectations were met
			mockLister.AssertExpectations(t)
		})
	}
}

func TestGetInstalledPluginInfoCommandField(t *testing.T) {
	// Store original os.Args[0] to restore later
	originalArgs0 := os.Args[0]
	defer func() {
		os.Args[0] = originalArgs0
	}()

	os.Args[0] = "/usr/local/bin/velero"

	mockLister := &MockPluginLister{}
	plugins := []framework.PluginIdentifier{
		{
			Name:    "velero.io/test",
			Kind:    common.PluginKindBackupItemAction,
			Command: "/usr/local/bin/velero",
		},
	}

	mockLister.On("List", common.PluginKindBackupItemAction).Return(plugins)

	// Set up mock expectations for all other plugin kinds
	allKinds := common.AllPluginKinds()
	for _, kind := range allKinds {
		if kind != common.PluginKindBackupItemAction {
			mockLister.On("List", kind).Return([]framework.PluginIdentifier{})
		}
	}

	result := GetInstalledPluginInfo(mockLister)

	assert.Len(t, result, 1)
	assert.Equal(t, "/usr/local/bin/velero", result[0].Command)
	assert.True(t, result[0].BuiltIn)
	assert.Equal(t, "velero.io/test", result[0].Name)
	assert.Equal(t, "BackupItemAction", result[0].Kind)

	mockLister.AssertExpectations(t)
}
