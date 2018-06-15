package restic

import (
	"encoding/json"
	"os/exec"

	"github.com/pkg/errors"
)

// GetSnapshotID runs a 'restic snapshots' command to get the ID of the snapshot
// in the specified repo matching the set of provided tags, or an error if a
// unique snapshot cannot be identified.
func GetSnapshotID(repoIdentifier, passwordFile string, tags map[string]string) (string, error) {
	output, err := GetSnapshotCommand(repoIdentifier, passwordFile, tags).Cmd().Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", errors.Wrapf(err, "error running command, stderr=%s", exitErr.Stderr)
		}
		return "", errors.Wrap(err, "error running command")
	}

	type snapshotID struct {
		ShortID string `json:"short_id"`
	}

	var snapshots []snapshotID
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return "", errors.Wrap(err, "error unmarshalling restic snapshots result")
	}

	if len(snapshots) != 1 {
		return "", errors.Errorf("expected one matching snapshot, got %d", len(snapshots))
	}

	return snapshots[0].ShortID, nil
}
