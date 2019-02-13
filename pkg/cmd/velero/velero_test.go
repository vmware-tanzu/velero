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

package velero

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func runVcmdNoWrap(command string, args ...string) (string, string, error) {
	var bout, berr bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	err := cmd.Run()
	stdout, stderr := bout.String(), berr.String()
	return stdout, stderr, err
}

// RunVcmd is a utility function for Velero testing that executes a specified command
func RunVcmd(command string, args ...string) (string, string, error) {
	stdout, stderr, err := runVcmdNoWrap(command, args...)
	if err != nil {
		return stdout, stderr, errors.Wrapf(err, "error running %s %v; \nstdout %q, \nstderr %q, \ngot error",
			command, args, stdout, stderr)
	}
	return stdout, stderr, nil
}

// RunSubVcommand is a utility function for Velero testing that executes a Cobra sub command
func RunSubVcommand(t *testing.T, subCmds []*cobra.Command, command string, args ...string) {
	subCmd := getSubCommand(t, subCmds, command)
	subCmd.SetArgs(args)
	if err := subCmd.Execute(); err != nil {
		t.Fatalf("Could not execute subcommand: %s", command)
	}
}

// AssertSubVcillabdHasFlags is a utility function for Velero testing that assert if a Cobra sub command has expected flags
func AssertSubVcommandHasFlags(t *testing.T, subCmds []*cobra.Command, command string, flags ...string) {
	subCmd := getSubCommand(t, subCmds, command)

	for _, flag := range flags {
		if subCmd.Flags().Lookup(flag) == nil {
			t.Errorf("Could not find expecte flag %s for command %s", flag, command)
		}
	}
}

// getSubVcommand is a utility function for Velero to get the list of sub commands
func getSubVcommand(t *testing.T, subCmds []*cobra.Command, name string) *cobra.Command {
	for _, subCmd := range subCmds {
		if subCmd.Name() == name {
			return subCmd
		}
	}
	t.Fatalf("Unable to find sub command %s", name)

	return nil
}
