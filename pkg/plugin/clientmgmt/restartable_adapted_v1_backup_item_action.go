/*
Copyright 2018, 2021 the Velero contributors.

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
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	backupitemactionv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/backupitemaction/v1"
)

type restartableAdaptedV1BackupItemAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
}

// newAdaptedV1BackupItemAction returns a new restartableAdaptedV1BackupItemAction.
func newAdaptedV1BackupItemAction(name string, sharedPluginProcess RestartableProcess) *restartableAdaptedV1BackupItemAction {
	r := &restartableAdaptedV1BackupItemAction{
		key:                 kindAndName{kind: framework.PluginKindBackupItemAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getBackupItemAction returns the backup item action for this restartableAdaptedV1BackupItemAction. It does *not* restart the
// plugin process.
func (r *restartableAdaptedV1BackupItemAction) getBackupItemAction() (backupitemactionv1.BackupItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	backupItemAction, ok := plugin.(backupitemactionv1.BackupItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a BackupItemAction!", plugin)
	}

	return backupItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the backup item action for this restartableAdaptedV1BackupItemAction.
func (r *restartableAdaptedV1BackupItemAction) getDelegate() (backupitemactionv1.BackupItemAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getBackupItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1BackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1BackupItemAction) Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}

	return delegate.Execute(item, backup)
}

// Version 2: simply discard ctx and call version 1 function.
// ExecuteV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1BackupItemAction) ExecuteV2(ctx context.Context, item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}
	return delegate.Execute(item, backup)
}
