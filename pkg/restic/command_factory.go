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
	"strings"
)

// BackupCommand returns a Command for running a restic backup.
func BackupCommand(repoPrefix, repo, passwordFile, path string, tags map[string]string) *Command {
	return &Command{
		Command:      "backup",
		RepoPrefix:   repoPrefix,
		Repo:         repo,
		PasswordFile: passwordFile,
		Dir:          path,
		Args:         []string{"."},
		ExtraFlags:   backupTagFlags(tags),
	}
}

func backupTagFlags(tags map[string]string) []string {
	var flags []string
	for k, v := range tags {
		flags = append(flags, fmt.Sprintf("--tag=%s=%s", k, v))
	}
	return flags
}

// RestoreCommand returns a Command for running a restic restore.
func RestoreCommand(repoPrefix, repo, passwordFile, snapshotID, target string) *Command {
	return &Command{
		Command:      "restore",
		RepoPrefix:   repoPrefix,
		Repo:         repo,
		PasswordFile: passwordFile,
		Dir:          target,
		Args:         []string{snapshotID},
		ExtraFlags:   []string{"--target=."},
	}
}

// GetSnapshotCommand returns a Command for running a restic (get) snapshots.
func GetSnapshotCommand(repoPrefix, repo, passwordFile string, tags map[string]string) *Command {
	return &Command{
		Command:      "snapshots",
		RepoPrefix:   repoPrefix,
		Repo:         repo,
		PasswordFile: passwordFile,
		ExtraFlags:   []string{"--json", "--last", getSnapshotTagFlag(tags)},
	}
}

func getSnapshotTagFlag(tags map[string]string) string {
	var tagFilters []string
	for k, v := range tags {
		tagFilters = append(tagFilters, fmt.Sprintf("%s=%s", k, v))
	}

	return fmt.Sprintf("--tag=%s", strings.Join(tagFilters, ","))
}

func InitCommand(repoPrefix, repo string) *Command {
	return &Command{
		Command:    "init",
		RepoPrefix: repoPrefix,
		Repo:       repo,
	}
}

func CheckCommand(repoPrefix, repo string) *Command {
	return &Command{
		Command:    "check",
		RepoPrefix: repoPrefix,
		Repo:       repo,
	}
}

func PruneCommand(repoPrefix, repo string) *Command {
	return &Command{
		Command:    "prune",
		RepoPrefix: repoPrefix,
		Repo:       repo,
	}
}

func ForgetCommand(repoPrefix, repo, snapshotID string) *Command {
	return &Command{
		Command:    "forget",
		RepoPrefix: repoPrefix,
		Repo:       repo,
		Args:       []string{snapshotID},
	}
}
