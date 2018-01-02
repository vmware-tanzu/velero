/*
Copyright 2017 the Heptio Ark contributors.

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
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

// GetOptionalStringFlag returns the value of the specified flag from a
// cobra command, or the zero value ("") if the flag was not specified.
func GetOptionalStringFlag(cmd *cobra.Command, flagName string) string {
	return GetStringFlag(cmd, flagName, false)
}

// GetStringFlag returns the value of the specified flag from a
// cobra command. If the flag is not specified and fatalIfMissing is true,
// this function logs a fatal error and calls os.Exit(255).
func GetStringFlag(cmd *cobra.Command, flagName string, fatalIfMissing bool) string {
	s, err := cmd.Flags().GetString(flagName)
	if err != nil && fatalIfMissing {
		glog.Fatalf("error accessing flag %q for command %s: %v", flagName, cmd.Name(), err)
	}
	return s
}

// GetOptionalBoolFlag returns the value of the specified flag from a
// cobra command, or the zero value (false) if the flag was not specified.
func GetOptionalBoolFlag(cmd *cobra.Command, flagName string) bool {
	return GetBoolFlag(cmd, flagName, false)
}

// GetBoolFlag returns the value of the specified flag from a
// cobra command. If the flag is not specified and fatalIfMissing is true,
// this function logs a fatal error and calls os.Exit(255).
func GetBoolFlag(cmd *cobra.Command, flagName string, fatalIfMissing bool) bool {
	b, err := cmd.Flags().GetBool(flagName)
	if err != nil && fatalIfMissing {
		glog.Fatalf("error accessing flag %q for command %s: %v", flagName, cmd.Name(), err)
	}
	return b
}

// GetOptionalStringArrayFlag returns the value of the specified flag from a
// cobra command, or the zero value if the flag was not specified.
func GetOptionalStringArrayFlag(cmd *cobra.Command, flagName string) []string {
	return GetStringArrayFlag(cmd, flagName, false)
}

// GetStringArrayFlag returns the value of the specified flag from a
// cobra command. If the flag is not specified and fatalIfMissing is true,
// this function logs a fatal error and calls os.Exit(255).
func GetStringArrayFlag(cmd *cobra.Command, flagName string, fatalIfMissing bool) []string {
	f := cmd.Flag(flagName)
	if f == nil {
		if fatalIfMissing {
			glog.Fatalf("error accessing flag %q for command %s: not specified", flagName, cmd.Name())
		}
		return []string{}
	}
	v := f.Value.(*StringArray)
	return *v
}
