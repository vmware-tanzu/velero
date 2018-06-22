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
