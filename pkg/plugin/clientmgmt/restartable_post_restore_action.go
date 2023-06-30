/*
Copyright 2020 the Velero contributors.

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

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type restartablePostRestoreAction struct {
	key                 process.KindAndName
	sharedPluginProcess process.RestartableProcess
}

func newRestartablePostRestoreAction(name string, sharedPluginProcess process.RestartableProcess) *restartablePostRestoreAction {
	return &restartablePostRestoreAction{
		key:                 process.KindAndName{Kind: common.PluginKindPostRestoreAction, Name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
}

func (r *restartablePostRestoreAction) getPostRestoreAction() (velero.PostRestoreAction, error) {
	plugin, err := r.sharedPluginProcess.GetByKindAndName(r.key)
	if err != nil {
		return nil, err
	}
	PostRestoreAction, ok := plugin.(velero.PostRestoreAction)
	if !ok {
		return nil, errors.Errorf("%T is not a PostRestoreAction", plugin)
	}
	return PostRestoreAction, nil
}

func (r *restartablePostRestoreAction) getDelegate() (velero.PostRestoreAction, error) {
	if err := r.sharedPluginProcess.ResetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getPostRestoreAction()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePostRestoreAction) Execute(restore *api.Restore) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.Execute(restore)
}
