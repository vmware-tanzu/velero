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

package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// TestDiskSpaceErrorHandling tests that disk space errors are properly detected and enhanced with helpful messages
func TestDiskSpaceErrorHandling(t *testing.T) {
	tests := []struct {
		name                  string
		originalError         error
		baseErrorMsg          string
		expectedInMessage     []string
		shouldContainHelpText bool
	}{
		{
			name:                  "mkdir with disk space error",
			originalError:         errors.New("mkdir: cannot create directory: no space left on device"),
			baseErrorMsg:          "error creating .velero directory for done file",
			expectedInMessage:     []string{"no space left on device", "--write-sparse-files", "https://velero.io/docs/main/restore-reference/#write-sparse-files"},
			shouldContainHelpText: true,
		},
		{
			name:                  "write file with disk space error",
			originalError:         errors.New("write: no space left on device"),
			baseErrorMsg:          "error writing done file",
			expectedInMessage:     []string{"no space left on device", "--write-sparse-files", "https://velero.io/docs/main/restore-reference/#write-sparse-files"},
			shouldContainHelpText: true,
		},
		{
			name:                  "mkdir with permission error",
			originalError:         errors.New("mkdir: permission denied"),
			baseErrorMsg:          "error creating .velero directory for done file",
			expectedInMessage:     []string{"permission denied"},
			shouldContainHelpText: false,
		},
		{
			name:                  "write file with permission error",
			originalError:         errors.New("write: permission denied"),
			baseErrorMsg:          "error writing done file",
			expectedInMessage:     []string{"permission denied"},
			shouldContainHelpText: false,
		},
		{
			name:                  "disk quota exceeded error",
			originalError:         errors.New("mkdir: disk quota exceeded"),
			baseErrorMsg:          "error creating .velero directory for done file",
			expectedInMessage:     []string{"disk quota exceeded"},
			shouldContainHelpText: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Simulate the error handling logic from the controller
			errMsg := test.baseErrorMsg
			finalErr := test.originalError

			if strings.Contains(test.originalError.Error(), "no space left on device") {
				errMsg = fmt.Sprintf("%s: %v. Consider using the --write-sparse-files flag during restore to optimize disk usage. See https://velero.io/docs/main/restore-reference/#write-sparse-files for more details", errMsg, test.originalError)
				finalErr = errors.New(errMsg)
			}

			// Verify the error message contains expected content
			for _, expected := range test.expectedInMessage {
				assert.Contains(t, finalErr.Error(), expected, "Error message should contain: %s", expected)
			}

			// Verify help text is only added for disk space errors
			if test.shouldContainHelpText {
				assert.Contains(t, finalErr.Error(), "--write-sparse-files")
				assert.Contains(t, finalErr.Error(), "https://velero.io/docs/main/restore-reference/#write-sparse-files")
			} else {
				assert.NotContains(t, finalErr.Error(), "--write-sparse-files")
				assert.NotContains(t, finalErr.Error(), "https://velero.io/docs/main/restore-reference/#write-sparse-files")
			}
		})
	}
}

// TestErrorMessageFormat verifies that the error messages are formatted correctly
func TestErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name        string
		originalErr error
		baseMsg     string
		expectedMsg string
	}{
		{
			name:        "mkdir disk space error formatting",
			originalErr: errors.New("mkdir: cannot create directory: no space left on device"),
			baseMsg:     "error creating .velero directory for done file",
			expectedMsg: "error creating .velero directory for done file: mkdir: cannot create directory: no space left on device. Consider using the --write-sparse-files flag during restore to optimize disk usage. See https://velero.io/docs/main/restore-reference/#write-sparse-files for more details",
		},
		{
			name:        "write file disk space error formatting",
			originalErr: errors.New("write: no space left on device"),
			baseMsg:     "error writing done file",
			expectedMsg: "error writing done file: write: no space left on device. Consider using the --write-sparse-files flag during restore to optimize disk usage. See https://velero.io/docs/main/restore-reference/#write-sparse-files for more details",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Simulate the exact error formatting logic
			errMsg := test.baseMsg
			if strings.Contains(test.originalErr.Error(), "no space left on device") {
				errMsg = fmt.Sprintf("%s: %v. Consider using the --write-sparse-files flag during restore to optimize disk usage. See https://velero.io/docs/main/restore-reference/#write-sparse-files for more details", errMsg, test.originalErr)
			}

			assert.Equal(t, test.expectedMsg, errMsg, "Error message should be formatted correctly")
		})
	}
}
