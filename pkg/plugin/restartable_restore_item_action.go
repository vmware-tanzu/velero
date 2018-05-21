package plugin

import (
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/restore"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// restartableRestoreItemAction is a restore item action for a given implementation (such as "pod"). It is associated with
// a restartablePluginProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableRestoreItemAction asks its restartablePluginProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableRestoreItemAction struct {
	key                 kindAndName
	sharedPluginProcess *restartablePluginProcess
	config              map[string]string
}

// newRestartableRestoreItemAction returns a new restartableRestoreItemAction.
func newRestartableRestoreItemAction(name string, sharedPluginProcess *restartablePluginProcess) *restartableRestoreItemAction {
	r := &restartableRestoreItemAction{
		key:                 kindAndName{kind: PluginKindRestoreItemAction, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getRestoreItemAction returns the restore item action for this restartableRestoreItemAction. It does *not* restart the
// plugin process.
func (r *restartableRestoreItemAction) getRestoreItemAction() (restore.ItemAction, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	restoreItemAction, ok := plugin.(restore.ItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a restore.ItemAction!", plugin)
	}

	return restoreItemAction, nil
}

// getDelegate restarts the plugin process (if needed) and returns the restore item action for this restartableRestoreItemAction.
func (r *restartableRestoreItemAction) getDelegate() (restore.ItemAction, error) {
	if _, err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getRestoreItemAction()
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableRestoreItemAction) AppliesTo() (restore.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return restore.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableRestoreItemAction) Execute(obj runtime.Unstructured, restore *api.Restore) (res runtime.Unstructured, warning error, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}

	return delegate.Execute(obj, restore)
}
