/*
Copyright The Velero Contributors.

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
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

const restoreProgressCheckInterval = 10 * time.Second
const backupProgressCheckInterval = 10 * time.Second

var fileSystem = filesystem.NewFileSystem()

type backupStatusLine struct {
	MessageType string `json:"message_type"`
	// seen in status lines
	TotalBytes int64 `json:"total_bytes"`
	BytesDone  int64 `json:"bytes_done"`
	// seen in summary line at the end
	TotalBytesProcessed int64 `json:"total_bytes_processed"`
}

// GetSnapshotID runs provided 'restic snapshots' command to get the ID of a snapshot
// and an error if a unique snapshot cannot be identified.
func GetSnapshotID(snapshotIdCmd *Command) (string, error) {
	stdout, stderr, err := exec.RunCommand(snapshotIdCmd.Cmd())
	if err != nil {
		return "", errors.Wrapf(err, "error running command, stderr=%s", stderr)
	}

	type snapshotID struct {
		ShortID string `json:"short_id"`
	}

	var snapshots []snapshotID
	if err := json.Unmarshal([]byte(stdout), &snapshots); err != nil {
		return "", errors.Wrap(err, "error unmarshalling restic snapshots result")
	}

	if len(snapshots) != 1 {
		return "", errors.Errorf("expected one matching snapshot by command: %s, got %d", snapshotIdCmd.String(), len(snapshots))
	}

	return snapshots[0].ShortID, nil
}

func DecodeBackupStatusLine(lastLine []byte) (backupStatusLine, error) {
	var stat backupStatusLine
	if err := json.Unmarshal(lastLine, &stat); err != nil {
		return stat, errors.Wrapf(err, "unable to decode backup JSON line: %s", string(lastLine))
	}
	return stat, nil
}

// GetLastLine returns the last line of a byte array. The string is assumed to
// have a newline at the end of it, so this returns the substring between the
// last two newlines.
func GetLastLine(b []byte) []byte {
	if b == nil || len(b) == 0 {
		return []byte("")
	}
	// subslice the byte array to ignore the newline at the end of the string
	lastNewLineIdx := bytes.LastIndex(b[:len(b)-1], []byte("\n"))
	return b[lastNewLineIdx+1 : len(b)-1]
}

// GetSummaryLine looks for the summary JSON line
// (`{"message_type:"summary",...`) in the restic backup command output. Due to
// an issue in Restic, this might not always be the last line
// (https://github.com/restic/restic/issues/2389). It returns an error if it
// can't be found.
func GetSummaryLine(b []byte) ([]byte, error) {
	summaryLineIdx := bytes.LastIndex(b, []byte(`{"message_type":"summary"`))
	if summaryLineIdx < 0 {
		return nil, errors.New("unable to find summary in restic backup command output")
	}
	// find the end of the summary line
	newLineIdx := bytes.Index(b[summaryLineIdx:], []byte("\n"))
	if newLineIdx < 0 {
		return nil, errors.New("unable to get summary line from restic backup command output")
	}
	return b[summaryLineIdx : summaryLineIdx+newLineIdx], nil
}

func GetSnapshotSize(repoIdentifier, passwordFile, caCertFile, snapshotID string, env []string, insecureTLS string) (int64, error) {
	cmd := StatsCommand(repoIdentifier, passwordFile, snapshotID)
	cmd.Env = env
	cmd.CACertFile = caCertFile

	if len(insecureTLS) > 0 {
		cmd.ExtraFlags = append(cmd.ExtraFlags, insecureTLS)
	}

	stdout, stderr, err := exec.RunCommand(cmd.Cmd())
	if err != nil {
		return 0, errors.Wrapf(err, "error running command, stderr=%s", stderr)
	}

	var snapshotStats struct {
		TotalSize int64 `json:"total_size"`
	}

	if err := json.Unmarshal([]byte(stdout), &snapshotStats); err != nil {
		return 0, errors.Wrapf(err, "error unmarshalling restic stats result, stdout=%s", stdout)
	}

	return snapshotStats.TotalSize, nil
}

func GetVolumeSize(path string) (int64, error) {
	var size int64

	files, err := fileSystem.ReadDir(path)
	if err != nil {
		return 0, errors.Wrapf(err, "error reading directory %s", path)
	}

	for _, file := range files {
		if file.IsDir() {
			s, err := GetVolumeSize(fmt.Sprintf("%s/%s", path, file.Name()))
			if err != nil {
				return 0, err
			}
			size += s
		} else {
			size += file.Size()
		}
	}

	return size, nil
}
