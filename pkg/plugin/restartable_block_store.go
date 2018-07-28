/*
Copyright 2018 the Heptio Ark contributors.

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
package plugin

import (
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// restartableBlockStore is an object store for a given implementation (such as "aws"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableBlockStore asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableBlockStore struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	config              map[string]string
}

// newRestartableBlockStore returns a new restartableBlockStore.
func newRestartableBlockStore(name string, sharedPluginProcess RestartableProcess) *restartableBlockStore {
	key := kindAndName{kind: PluginKindBlockStore, name: name}
	r := &restartableBlockStore{
		key:                 key,
		sharedPluginProcess: sharedPluginProcess,
	}

	// Register our reinitializer so we can reinitialize after a restart with r.config.
	sharedPluginProcess.addReinitializer(key, r)

	return r
}

// reinitialize reinitializes a re-dispensed plugin using the initial data passed to Init().
func (r *restartableBlockStore) reinitialize(dispensed interface{}) error {
	blockStore, ok := dispensed.(cloudprovider.BlockStore)
	if !ok {
		return errors.Errorf("%T is not a cloudprovider.BlockStore!", dispensed)
	}
	return r.init(blockStore, r.config)
}

// getBlockStore returns the block store for this restartableBlockStore. It does *not* restart the
// plugin process.
func (r *restartableBlockStore) getBlockStore() (cloudprovider.BlockStore, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	blockStore, ok := plugin.(cloudprovider.BlockStore)
	if !ok {
		return nil, errors.Errorf("%T is not a cloudprovider.BlockStore!", plugin)
	}

	return blockStore, nil
}

// getDelegate restarts the plugin process (if needed) and returns the block store for this restartableBlockStore.
func (r *restartableBlockStore) getDelegate() (cloudprovider.BlockStore, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getBlockStore()
}

// Init initializes the block store instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableBlockStore) Init(config map[string]string) error {
	if r.config != nil {
		return errors.Errorf("already initialized")
	}

	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getBlockStore()
	if err != nil {
		return err
	}

	r.config = config

	return r.init(delegate, config)
}

// init calls Init on blockStore with config. This is split out from Init() so that both Init() and reinitialize() may
// call it using a specific BlockStore.
func (r *restartableBlockStore) init(blockStore cloudprovider.BlockStore, config map[string]string) error {
	return blockStore.Init(config)
}

// CreateVolumeFromSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) CreateVolumeFromSnapshot(snapshotID string, volumeType string, volumeAZ string, iops *int64) (volumeID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
}

// GetVolumeID restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.GetVolumeID(pv)
}

// SetVolumeID restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.SetVolumeID(pv, volumeID)
}

// GetVolumeInfo restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) GetVolumeInfo(volumeID string, volumeAZ string) (string, *int64, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", nil, err
	}
	return delegate.GetVolumeInfo(volumeID, volumeAZ)
}

// CreateSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) CreateSnapshot(volumeID string, volumeAZ string, tags map[string]string) (snapshotID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSnapshot(volumeID, volumeAZ, tags)
}

// DeleteSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableBlockStore) DeleteSnapshot(snapshotID string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteSnapshot(snapshotID)
}
