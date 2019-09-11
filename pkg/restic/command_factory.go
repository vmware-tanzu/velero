/*
Copyright 2018, 2019 the Velero contributors.

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
func BackupCommand(repoIdentifier, passwordFile, path string, tags map[string]string) *Command {
	// --host flag is provided with a generic value because restic uses the host
	// to find a parent snapshot, and by default it will be the name of the daemonset pod
	// where the `restic backup` command is run. If this pod is recreated, we want to continue
	// taking incremental backups rather than triggering a full one due to a new pod name.

	return &Command{
		Command:        "backup",
		RepoIdentifier: repoIdentifier,
		PasswordFile:   passwordFile,
		Dir:            path,
		Args:           []string{"."},
		ExtraFlags:     append(backupTagFlags(tags), "--host=velero", "--json"),
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
func RestoreCommand(repoIdentifier, passwordFile, snapshotID, target string) *Command {
	return &Command{
		Command:        "restore",
		RepoIdentifier: repoIdentifier,
		PasswordFile:   passwordFile,
		Dir:            target,
		Args:           []string{snapshotID},
		ExtraFlags:     []string{"--target=."},
	}
}

// GetSnapshotCommand returns a Command for running a restic (get) snapshots.
func GetSnapshotCommand(repoIdentifier, passwordFile string, tags map[string]string) *Command {
	return &Command{
		Command:        "snapshots",
		RepoIdentifier: repoIdentifier,
		PasswordFile:   passwordFile,
		ExtraFlags:     []string{"--json", "--last", getSnapshotTagFlag(tags)},
	}
}

func getSnapshotTagFlag(tags map[string]string) string {
	var tagFilters []string
	for k, v := range tags {
		tagFilters = append(tagFilters, fmt.Sprintf("%s=%s", k, v))
	}

	return fmt.Sprintf("--tag=%s", strings.Join(tagFilters, ","))
}

func InitCommand(repoIdentifier string) *Command {
	return &Command{
		Command:        "init",
		RepoIdentifier: repoIdentifier,
	}
}

func SnapshotsCommand(repoIdentifier string) *Command {
	return &Command{
		Command:        "snapshots",
		RepoIdentifier: repoIdentifier,
	}
}

func PruneCommand(repoIdentifier string) *Command {
	return &Command{
		Command:        "prune",
		RepoIdentifier: repoIdentifier,
	}
}

func ForgetCommand(repoIdentifier, snapshotID string) *Command {
	return &Command{
		Command:        "forget",
		RepoIdentifier: repoIdentifier,
		Args:           []string{snapshotID},
	}
}

func UnlockCommand(repoIdentifier string) *Command {
	return &Command{
		Command:        "unlock",
		RepoIdentifier: repoIdentifier,
	}
}

func StatsCommand(repoIdentifier, passwordFile, snapshotID string) *Command {
	return &Command{
		Command:        "stats",
		RepoIdentifier: repoIdentifier,
		PasswordFile:   passwordFile,
		Args:           []string{snapshotID},
		ExtraFlags:     []string{"--json"},
	}
}
