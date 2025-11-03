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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// GetProgramName returns the executable name (e.g., "velero" or custom binary name).
// It uses os.Executable which returns the path name for the executable that started
// the current process.
func GetProgramName(cmd *cobra.Command) string {
	executable, err := os.Executable()
	if err == nil {
		return filepath.Base(executable)
	}

	// Fallback to "velero" if we can't get the executable name
	return "velero"
}
