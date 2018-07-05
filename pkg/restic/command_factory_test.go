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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupCommand(t *testing.T) {
	c := BackupCommand("repo-id", "password-file", "path", map[string]string{"foo": "bar", "c": "d"})

	assert.Equal(t, "backup", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
	assert.Equal(t, "password-file", c.PasswordFile)
	assert.Equal(t, "path", c.Dir)
	assert.Equal(t, []string{"."}, c.Args)

	expected := []string{"--tag=foo=bar", "--tag=c=d", "--hostname=ark"}
	sort.Strings(expected)
	sort.Strings(c.ExtraFlags)
	assert.Equal(t, expected, c.ExtraFlags)
}

func TestRestoreCommand(t *testing.T) {
	c := RestoreCommand("repo-id", "password-file", "snapshot-id", "target")

	assert.Equal(t, "restore", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
	assert.Equal(t, "password-file", c.PasswordFile)
	assert.Equal(t, "target", c.Dir)
	assert.Equal(t, []string{"snapshot-id"}, c.Args)
	assert.Equal(t, []string{"--target=."}, c.ExtraFlags)
}

func TestGetSnapshotCommand(t *testing.T) {
	c := GetSnapshotCommand("repo-id", "password-file", map[string]string{"foo": "bar", "c": "d"})

	assert.Equal(t, "snapshots", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
	assert.Equal(t, "password-file", c.PasswordFile)

	expected := []string{"--json", "--last", "--tag=foo=bar,c=d"}
	sort.Strings(expected)
	sort.Strings(c.ExtraFlags)
	assert.Equal(t, expected, c.ExtraFlags)
}

func TestInitCommand(t *testing.T) {
	c := InitCommand("repo-id")

	assert.Equal(t, "init", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
}

func TestCheckCommand(t *testing.T) {
	c := CheckCommand("repo-id")

	assert.Equal(t, "check", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
}

func TestPruneCommand(t *testing.T) {
	c := PruneCommand("repo-id")

	assert.Equal(t, "prune", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
}

func TestForgetCommand(t *testing.T) {
	c := ForgetCommand("repo-id", "snapshot-id")

	assert.Equal(t, "forget", c.Command)
	assert.Equal(t, "repo-id", c.RepoIdentifier)
	assert.Equal(t, []string{"snapshot-id"}, c.Args)
}
