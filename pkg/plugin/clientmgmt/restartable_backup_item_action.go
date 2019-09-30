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

package clientmgmt

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// restartableBackupItemAction is a backup item action for a given implementation (such as "pod"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableBackupItemAction asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableBackupItemAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
}

// newRestartableBackupItemAction returns a new restartableBackupItemAction.
func newRestartableBackupItemAction(name string, sharedPluginProcess RestartableProcess) *restartableBackupItemAction {
	r := &restartableBackupItemAction{
		key:                 kindAndName{kind: framework.PluginKindBackupItemAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getBackupItemAction returns the backup item action for this restartableBackupItemAction. It does *not* restart the
// plugin process.
func (r *restartableBackupItemAction) getBackupItemAction() (velero.BackupItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	backupItemAction, ok := plugin.(velero.BackupItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a BackupItemAction!", plugin)
	}

	return backupItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the backup item action for this restartableBackupItemAction.
func (r *restartableBackupItemAction) getDelegate() (velero.BackupItemAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getBackupItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableBackupItemAction) Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}

	return delegate.Execute(item, backup)
}
