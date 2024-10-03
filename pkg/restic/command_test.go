/*
Copyright 2020 the Velero contributors.

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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoName(t *testing.T) {
	c := &Command{RepoIdentifier: ""}
	assert.Equal(t, "", c.RepoName())

	c.RepoIdentifier = "s3:s3.amazonaws.com/bucket/prefix/repo"
	assert.Equal(t, "repo", c.RepoName())

	c.RepoIdentifier = "azure:bucket:/repo"
	assert.Equal(t, "repo", c.RepoName())

	c.RepoIdentifier = "gs:bucket:/prefix/repo"
	assert.Equal(t, "repo", c.RepoName())
}

func TestStringSlice(t *testing.T) {
	c := &Command{
		Command:        "cmd",
		RepoIdentifier: "repo-id",
		PasswordFile:   "/path/to/password-file",
		Dir:            "/some/pwd",
		Args:           []string{"arg-1", "arg-2"},
		ExtraFlags:     []string{"--foo=bar"},
	}

	require.NoError(t, os.Unsetenv("VELERO_SCRATCH_DIR"))
	assert.Equal(t, []string{
		"restic",
		"cmd",
		"--repo=repo-id",
		"--password-file=/path/to/password-file",
		"arg-1",
		"arg-2",
		"--foo=bar",
	}, c.StringSlice())

	os.Setenv("VELERO_SCRATCH_DIR", "/foo")
	assert.Equal(t, []string{
		"restic",
		"cmd",
		"--repo=repo-id",
		"--password-file=/path/to/password-file",
		"--cache-dir=/foo/.cache/restic",
		"arg-1",
		"arg-2",
		"--foo=bar",
	}, c.StringSlice())

	require.NoError(t, os.Unsetenv("VELERO_SCRATCH_DIR"))
}

func TestString(t *testing.T) {
	c := &Command{
		Command:        "cmd",
		RepoIdentifier: "repo-id",
		PasswordFile:   "/path/to/password-file",
		Dir:            "/some/pwd",
		Args:           []string{"arg-1", "arg-2"},
		ExtraFlags:     []string{"--foo=bar"},
	}

	require.NoError(t, os.Unsetenv("VELERO_SCRATCH_DIR"))
	assert.Equal(t, "restic cmd --repo=repo-id --password-file=/path/to/password-file arg-1 arg-2 --foo=bar", c.String())
}

func TestCmd(t *testing.T) {
	c := &Command{
		Command:        "cmd",
		RepoIdentifier: "repo-id",
		PasswordFile:   "/path/to/password-file",
		Dir:            "/some/pwd",
		Args:           []string{"arg-1", "arg-2"},
		ExtraFlags:     []string{"--foo=bar"},
	}

	require.NoError(t, os.Unsetenv("VELERO_SCRATCH_DIR"))
	execCmd := c.Cmd()

	assert.Equal(t, c.StringSlice(), execCmd.Args)
	assert.Equal(t, c.Dir, execCmd.Dir)
}
