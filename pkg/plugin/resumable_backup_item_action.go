package plugin

import (
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type resumableBackupItemAction struct {
	key     kindAndName
	wrapper *wrapper
	config  map[string]string
}

func newResumableBackupItemAction(name string, wrapper *wrapper) *resumableBackupItemAction {
	key := kindAndName{kind: PluginKindBackupItemAction, name: name}
	r := &resumableBackupItemAction{
		key:     key,
		wrapper: wrapper,
	}
	return r
}

func (r *resumableBackupItemAction) getBackupItemAction() (backup.ItemAction, error) {
	plugin, err := r.wrapper.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	backupItemAction, ok := plugin.(backup.ItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not an ItemAction!", plugin)
	}

	return backupItemAction, nil
}

func (r *resumableBackupItemAction) getDelegate() (backup.ItemAction, error) {
	if _, err := r.wrapper.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getBackupItemAction()
}
func (r *resumableBackupItemAction) AppliesTo() (backup.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return backup.ResourceSelector{}, err
	}
	return delegate.AppliesTo()
}

func (r *resumableBackupItemAction) Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []backup.ResourceIdentifier, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}
	return delegate.Execute(item, backup)
}
