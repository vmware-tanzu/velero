/*
Copyright 2021 the Velero contributors.

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
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	restoreitemactionv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/restoreitemaction/v2"
)

// restartableRestoreItemAction is a restore item action for a given implementation (such as "pod"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableRestoreItemAction asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableRestoreItemAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	config              map[string]string
}

// newRestartableRestoreItemActionV2 returns a new restartableRestoreItemAction.
func newRestartableRestoreItemActionV2(
	name string, sharedPluginProcess RestartableProcess) restoreitemactionv2.RestoreItemAction {
	r := &restartableRestoreItemAction{
		key:                 kindAndName{kind: framework.PluginKindRestoreItemActionV2, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getRestoreItemAction returns the restore item action for this restartableRestoreItemAction. It does *not* restart the
// plugin process.
func (r *restartableRestoreItemAction) getRestoreItemAction() (restoreitemactionv2.RestoreItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	restoreItemAction, ok := plugin.(restoreitemactionv2.RestoreItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a RestoreItemAction!", plugin)
	}

	return restoreItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the restore item action for this restartableRestoreItemAction.
func (r *restartableRestoreItemAction) getDelegate() (restoreitemactionv2.RestoreItemAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getRestoreItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.Execute(input)
}

// ExecuteV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableRestoreItemAction) ExecuteV2(
	ctx context.Context, input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.ExecuteV2(ctx, input)
}
