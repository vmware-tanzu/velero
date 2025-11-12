/*
Copyright The Velero Contributors.

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

package util

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetProgramName(t *testing.T) {
	// Since the simplified implementation always uses os.Executable(),
	// all tests will return the test binary name
	tests := []struct {
		name        string
		cmd         *cobra.Command
		expected    string
		description string
	}{
		{
			name:        "returns test binary name",
			cmd:         &cobra.Command{},
			expected:    "util.test",
			description: "Should return the test binary name from os.Executable()",
		},
		{
			name:        "returns test binary name with nil command",
			cmd:         nil,
			expected:    "util.test",
			description: "Should return the test binary name even with nil command",
		},
		{
			name: "returns test binary name regardless of command hierarchy",
			cmd: func() *cobra.Command {
				root := &cobra.Command{Use: "custom-velero"}
				child := &cobra.Command{Use: "backup"}
				root.AddCommand(child)
				return child
			}(),
			expected:    "util.test",
			description: "Should always use os.Executable() regardless of command structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProgramName(tt.cmd)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestGetProgramNameWithRealExecutable(t *testing.T) {
	// This test verifies that GetProgramName always returns the executable name
	cmd := &cobra.Command{}
	result := GetProgramName(cmd)

	// The result should be the base name of the test binary
	// We don't assert a specific name since it varies based on how tests are run
	assert.NotEmpty(t, result, "Should return a non-empty program name")
	assert.NotContains(t, result, "/", "Should not contain path separators")
	assert.NotContains(t, result, "\\", "Should not contain Windows path separators")
}
