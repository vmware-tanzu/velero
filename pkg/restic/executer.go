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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// BackupExecuter ...
type BackupExecuter interface {
	RunBackup(interface{}, logrus.FieldLogger, func(velerov1api.PodVolumeOperationProgress)) (string, string, error)
	GetSnapshotID(interface{}) (string, error)
}

// BackupExec ...
type BackupExec struct{}

// RunBackup is a wrapper for the restic.RunBackup function in order to allow
// test.FakeResticBackupExec to be passed in for testing purposes.
func (exec BackupExec) RunBackup(command interface{}, log logrus.FieldLogger, updateFn func(velerov1api.PodVolumeOperationProgress)) (string, string, error) {
	cmd, ok := command.(*Command)
	if !ok {
		return "", "", errors.New("expecting command to be of type *restic.Command")
	}
	return RunBackup(cmd, log, updateFn)
}

// GetSnapshotID ...
func (exec BackupExec) GetSnapshotID(snapshotIdCmd interface{}) (string, error) {
	cmd, ok := snapshotIdCmd.(*Command)
	if !ok {
		return "", errors.New("expecting command to be of type *restic.Command")
	}

	return GetSnapshotID(cmd)
}
