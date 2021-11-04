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
	deleteitemactionv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/deleteitemaction/v1"
	deleteitemactionv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/deleteitemaction/v2"
)

type restartableAdaptedV1DeleteItemAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	config              map[string]string
}

// newAdaptedV1DeleteItemAction returns a new restartableAdaptedV1DeleteItemAction.
func newAdaptedV1DeleteItemAction(
	name string, sharedPluginProcess RestartableProcess) deleteitemactionv2.DeleteItemAction {
	r := &restartableAdaptedV1DeleteItemAction{
		key:                 kindAndName{kind: framework.PluginKindDeleteItemAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getDeleteItemAction returns the delete item action for this restartableDeleteItemAction.
// It does *not* restart the plugin process.
func (r *restartableAdaptedV1DeleteItemAction) getDeleteItemAction() (deleteitemactionv1.DeleteItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	deleteItemAction, ok := plugin.(deleteitemactionv1.DeleteItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a DeleteItemAction!", plugin)
	}

	return deleteItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the delete item action for this restartableDeleteItemAction.
func (r *restartableAdaptedV1DeleteItemAction) getDelegate() (deleteitemactionv1.DeleteItemAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getDeleteItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1DeleteItemAction) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1DeleteItemAction) Execute(input *velero.DeleteItemActionExecuteInput) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.Execute(input)
}

// ExecuteV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1DeleteItemAction) ExecuteV2(
	ctx context.Context, input *velero.DeleteItemActionExecuteInput) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.Execute(input)
}
