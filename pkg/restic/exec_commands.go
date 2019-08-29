/*
Copyright 2018 the Velero contributors.

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
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/util/exec"
)

const backupProgressCheckInterval = 10 * time.Second

// GetSnapshotID runs a 'restic snapshots' command to get the ID of the snapshot
// in the specified repo matching the set of provided tags, or an error if a
// unique snapshot cannot be identified.
func GetSnapshotID(repoIdentifier, passwordFile string, tags map[string]string, env []string) (string, error) {
	cmd := GetSnapshotCommand(repoIdentifier, passwordFile, tags)
	if len(env) > 0 {
		cmd.Env = env
	}

	stdout, stderr, err := exec.RunCommand(cmd.Cmd())
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
		return "", errors.Errorf("expected one matching snapshot, got %d", len(snapshots))
	}

	return snapshots[0].ShortID, nil
}

// RunBackup runs a `restic backup` command and watches the output to provide
// progress updates to the caller.
func RunBackup(backupCmd *Command, log logrus.FieldLogger, updateFunc func(velerov1api.PodVolumeOperationProgress)) (string, string, error) {
	// buffers for copying command stdout/err output into
	stdoutBuf := new(bytes.Buffer)
	stdoutStatBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	// stdoutWriter is used to copy the command's stdout to both stdoutStatBuf and
	// stdoutBuf, the former buffer is used to scan through the output and the
	// latter is used to return the command's output as a string
	stdoutWriter := io.MultiWriter(stdoutStatBuf, stdoutBuf)

	// create a channel to signal when to end the goroutine scanning for progress
	// updates
	quit := make(chan struct{})
	var wg sync.WaitGroup

	cmd := backupCmd.Cmd()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderr, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}

	cmd.Start()

	// copy command's stdout/err to buffers
	wg.Add(1)
	go func() {
		io.Copy(stdoutWriter, stdout)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		io.Copy(stderrBuf, stderr)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		// track the number of bytes we've read so far
		readBytes := 0
		ticker := time.NewTicker(backupProgressCheckInterval)
		for {
			select {
			case <-ticker.C:
				// skip what we've already read
				stdoutStatBuf.Next(readBytes)
				unreadBytes := stdoutStatBuf.Len()

				// scan through unread bytes by line, and record the last line
				scanner := bufio.NewScanner(stdoutStatBuf)
				lastLine := []byte{}
				for scanner.Scan() {
					lastLine = scanner.Bytes()
				}

				var stat velerov1api.PodVolumeOperationProgress
				if err := json.Unmarshal(lastLine, &stat); err != nil {
					log.WithError(errors.WithStack(err)).Errorf("unable to decode backup JSON line")
					continue
				}

				// if the line contains a non-empty bytes_done field, we can update the
				// caller with the progress
				if stat.BytesDone != 0 {
					updateFunc(stat)
				}

				readBytes = readBytes + unreadBytes
			case <-quit:
				ticker.Stop()
				wg.Done()
				return
			}
		}
	}()

	cmd.Wait()
	close(quit)
	wg.Wait()
	return stdoutBuf.String(), stderrBuf.String(), nil
}
