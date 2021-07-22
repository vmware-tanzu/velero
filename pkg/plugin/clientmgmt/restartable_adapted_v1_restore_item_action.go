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
	restoreitemactionv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/restoreitemaction/v1"
)

type restartableAdaptedV1RestoreItemAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	config              map[string]string
}

// newRestartableRestoreItemAction returns a new restartableRestoreItemAction.
func newAdaptedV1RestoreItemAction(name string, sharedPluginProcess RestartableProcess) *restartableAdaptedV1RestoreItemAction {
	r := &restartableAdaptedV1RestoreItemAction{
		key:                 kindAndName{kind: framework.PluginKindRestoreItemAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getRestoreItemAction returns the restore item action for this restartableRestoreItemAction. It does *not* restart the
// plugin process.
func (r *restartableAdaptedV1RestoreItemAction) getRestoreItemAction() (restoreitemactionv1.RestoreItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	restoreItemAction, ok := plugin.(restoreitemactionv1.RestoreItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a RestoreItemAction!", plugin)
	}

	return restoreItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the restore item action for this restartableRestoreItemAction.
func (r *restartableAdaptedV1RestoreItemAction) getDelegate() (restoreitemactionv1.RestoreItemAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getRestoreItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1RestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1RestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.Execute(input)
}

// ExecuteV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1RestoreItemAction) ExecuteV2(
	ctx context.Context, input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.Execute(input)
}
