package plugin

import (
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/restore"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type resumableRestoreItemAction struct {
	key     kindAndName
	wrapper *wrapper
	config  map[string]string
}

func newResumableRestoreItemAction(name string, wrapper *wrapper) *resumableRestoreItemAction {
	key := kindAndName{kind: PluginKindRestoreItemAction, name: name}
	r := &resumableRestoreItemAction{
		key:     key,
		wrapper: wrapper,
	}
	return r
}

func (r *resumableRestoreItemAction) getRestoreItemAction() (restore.ItemAction, error) {
	plugin, err := r.wrapper.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	restoreItemAction, ok := plugin.(restore.ItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not an ItemAction!", plugin)
	}

	return restoreItemAction, nil
}

func (r *resumableRestoreItemAction) getDelegate() (restore.ItemAction, error) {
	if _, err := r.wrapper.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getRestoreItemAction()
}
func (r *resumableRestoreItemAction) AppliesTo() (restore.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return restore.ResourceSelector{}, err
	}
	return delegate.AppliesTo()
}

func (r *resumableRestoreItemAction) Execute(obj runtime.Unstructured, restore *api.Restore) (res runtime.Unstructured, warning error, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, nil, err
	}
	return delegate.Execute(obj, restore)
}
