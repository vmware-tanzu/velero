/*
Copyright the Velero contributors.

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

	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type restartableItemSnapshotter struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
}

// newRestartableItemSnapshotter returns a new newRestartableItemSnapshotter.
func newRestartableItemSnapshotter(name string, sharedPluginProcess RestartableProcess) *restartableItemSnapshotter {
	r := &restartableItemSnapshotter{
		key:                 kindAndName{kind: framework.PluginKindItemSnapshotter, name: name},
		sharedPluginProcess: sharedPluginProcess,
	}
	return r
}

// getItemSnapshotter returns the item snapshotter for this restartableItemSnapshotter. It does *not* restart the
// plugin process.
func (r *restartableItemSnapshotter) getItemSnapshotter() (isv1.ItemSnapshotter, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	itemSnapshotter, ok := plugin.(isv1.ItemSnapshotter)
	if !ok {
		return nil, errors.Errorf("%T is not an ItemSnapshotter!", plugin)
	}

	return itemSnapshotter, nil
}

// getDelegate restarts the plugin process (if needed) and returns the item snapshotter for this restartableItemSnapshotter.
func (r *restartableItemSnapshotter) getDelegate() (isv1.ItemSnapshotter, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getItemSnapshotter()
}

func (r *restartableItemSnapshotter) Init(config map[string]string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.Init(config)
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableItemSnapshotter) AppliesTo() (velero.ResourceSelector, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return velero.ResourceSelector{}, err
	}

	return delegate.AppliesTo()
}

func (r *restartableItemSnapshotter) AlsoHandles(input *isv1.AlsoHandlesInput) ([]velero.ResourceIdentifier, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.AlsoHandles(input)
}

func (r *restartableItemSnapshotter) SnapshotItem(ctx context.Context, input *isv1.SnapshotItemInput) (*isv1.SnapshotItemOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.SnapshotItem(ctx, input)
}

func (r *restartableItemSnapshotter) Progress(input *isv1.ProgressInput) (*isv1.ProgressOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.Progress(input)
}

func (r *restartableItemSnapshotter) DeleteSnapshot(ctx context.Context, input *isv1.DeleteSnapshotInput) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}

	return delegate.DeleteSnapshot(ctx, input)
}

func (r *restartableItemSnapshotter) CreateItemFromSnapshot(ctx context.Context, input *isv1.CreateItemInput) (*isv1.CreateItemOutput, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}

	return delegate.CreateItemFromSnapshot(ctx, input)
}
