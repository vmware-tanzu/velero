/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Command represents a restic command.
type Command struct {
	Command        string
	RepoIdentifier string
	PasswordFile   string
	Dir            string
	Args           []string
	ExtraFlags     []string
}

func (c *Command) RepoName() string {
	if c.RepoIdentifier == "" {
		return ""
	}

	return c.RepoIdentifier[strings.LastIndex(c.RepoIdentifier, "/")+1:]
}

// StringSlice returns the command as a slice of strings.
func (c *Command) StringSlice() []string {
	res := []string{"restic"}

	res = append(res, c.Command, repoFlag(c.RepoIdentifier))
	if c.PasswordFile != "" {
		res = append(res, passwordFlag(c.PasswordFile))
	}

	// If ARK_SCRATCH_DIR is defined, put the restic cache within it. If not,
	// allow restic to choose the location. This makes running either in-cluster
	// or local (dev) work properly.
	if scratch := os.Getenv("ARK_SCRATCH_DIR"); scratch != "" {
		res = append(res, cacheDirFlag(filepath.Join(scratch, ".cache", "restic")))
	}

	res = append(res, c.Args...)
	res = append(res, c.ExtraFlags...)

	return res
}

// String returns the command as a string.
func (c *Command) String() string {
	return strings.Join(c.StringSlice(), " ")
}

// Cmd returns an exec.Cmd for the command.
func (c *Command) Cmd() *exec.Cmd {
	parts := c.StringSlice()
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = c.Dir

	return cmd
}

func repoFlag(repoIdentifier string) string {
	return fmt.Sprintf("--repo=%s", repoIdentifier)
}

func passwordFlag(file string) string {
	return fmt.Sprintf("--password-file=%s", file)
}

func cacheDirFlag(dir string) string {
	return fmt.Sprintf("--cache-dir=%s", dir)
}
