/*
Copyright The Velero contributors.

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
	"github.com/sirupsen/logrus"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// BackupExecuter ...
type BackupExecuter interface {
	RunBackup(*Command, logrus.FieldLogger, func(velerov1api.PodVolumeOperationProgress)) (string, string, error)
	GetSnapshotID(string, string, map[string]string, []string, string) (string, error)
}

// Backup ...
type BackupExec struct{}

// RunBackup ...
func (exec BackupExec) RunBackup(cmd *Command, log logrus.FieldLogger, updateFn func(velerov1api.PodVolumeOperationProgress)) (string, string, error) {
	return RunBackup(cmd, log, updateFn)
}

// GetSnapshotID ...
func (exec BackupExec) GetSnapshotID(repoID, pwFile string, tags map[string]string, env []string, caCertFile string) (string, error) {
	return GetSnapshotID(repoID, pwFile, tags, env, caCertFile)
}
