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

package clientmgmt

import (
	"github.com/pkg/errors"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type restartablePostBackupAction struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
}

func newRestartablePostBackupAction(name string, sharedPluginProcess RestartableProcess) *restartablePostBackupAction {
	r := &restartablePostBackupAction{
		key:                 kindAndName{kind: framework.PluginKindPostBackupAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

func (r *restartablePostBackupAction) getPostBackupAction() (velero.PostBackupAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	postBackupAction, ok := plugin.(velero.PostBackupAction)
	if !ok {
		return nil, errors.Errorf("%T is not a PostBackupAction!", plugin)
	}

	return postBackupAction, nil
}

func (r *restartablePostBackupAction) getDelegate() (velero.PostBackupAction, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getPostBackupAction()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePostBackupAction) Execute(backup *api.Backup) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.Execute(backup)
}
