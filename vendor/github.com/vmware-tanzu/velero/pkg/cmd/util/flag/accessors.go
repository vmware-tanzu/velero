/*
Copyright 2017, 2019 the Velero contributors.

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

package flag

import (
	"github.com/spf13/cobra"
)

// GetOptionalStringFlag returns the value of the specified flag from a
// cobra command, or the zero value ("") if the flag was not specified.
func GetOptionalStringFlag(cmd *cobra.Command, flagName string) string {
	s, _ := cmd.Flags().GetString(flagName)
	return s
}

// GetOptionalBoolFlag returns the value of the specified flag from a
// cobra command, or the zero value (false) if the flag was not specified.
func GetOptionalBoolFlag(cmd *cobra.Command, flagName string) bool {
	b, _ := cmd.Flags().GetBool(flagName)
	return b
}

// GetOptionalStringArrayFlag returns the value of the specified flag from a
// cobra command, or the zero value if the flag was not specified.
func GetOptionalStringArrayFlag(cmd *cobra.Command, flagName string) []string {
	f := cmd.Flag(flagName)
	if f == nil {
		return []string{}
	}
	v := f.Value.(*StringArray)
	return *v
}
