package restic

import (
	"fmt"
	"strings"
)

// BackupCommand returns a Command for running a restic backup.
func BackupCommand(repoIdentifier, passwordFile, path string, tags map[string]string) *Command {
	return &Command{
		Command:        "backup",
		RepoIdentifier: repoIdentifier,
		PasswordFile:   passwordFile,
		Dir:            path,
		Args:           []string{"."},
		ExtraFlags:     backupTagFlags(tags),
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

func CheckCommand(repoIdentifier string) *Command {
	return &Command{
		Command:        "check",
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
