/*
Copyright 2018 the Velero contributors.

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
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// restartableVolumeSnapshotter is a volume snapshotter for a given implementation (such as "aws"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableVolumeSnapshotter asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableVolumeSnapshotter struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	config              map[string]string
}

// newRestartableVolumeSnapshotter returns a new restartableVolumeSnapshotter.
func newRestartableVolumeSnapshotter(name string, sharedPluginProcess RestartableProcess) *restartableVolumeSnapshotter {
	key := kindAndName{kind: framework.PluginKindVolumeSnapshotter, name: name}
	r := &restartableVolumeSnapshotter{
		key:                 key,
		sharedPluginProcess: sharedPluginProcess,
	}

	// Register our reinitializer so we can reinitialize after a restart with r.config.
	sharedPluginProcess.addReinitializer(key, r)

	return r
}

// reinitialize reinitializes a re-dispensed plugin using the initial data passed to Init().
func (r *restartableVolumeSnapshotter) reinitialize(dispensed interface{}) error {
	volumeSnapshotter, ok := dispensed.(velero.VolumeSnapshotter)
	if !ok {
		return errors.Errorf("%T is not a VolumeSnapshotter!", dispensed)
	}
	return r.init(volumeSnapshotter, r.config)
}

// getVolumeSnapshotter returns the volume snapshotter for this restartableVolumeSnapshotter. It does *not* restart the
// plugin process.
func (r *restartableVolumeSnapshotter) getVolumeSnapshotter() (velero.VolumeSnapshotter, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	volumeSnapshotter, ok := plugin.(velero.VolumeSnapshotter)
	if !ok {
		return nil, errors.Errorf("%T is not a VolumeSnapshotter!", plugin)
	}

	return volumeSnapshotter, nil
}

// getDelegate restarts the plugin process (if needed) and returns the volume snapshotter for this restartableVolumeSnapshotter.
func (r *restartableVolumeSnapshotter) getDelegate() (velero.VolumeSnapshotter, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getVolumeSnapshotter()
}

// Init initializes the volume snapshotter instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableVolumeSnapshotter) Init(config map[string]string) error {
	if r.config != nil {
		return errors.Errorf("already initialized")
	}

	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getVolumeSnapshotter()
	if err != nil {
		return err
	}

	r.config = config

	return r.init(delegate, config)
}

// init calls Init on volumeSnapshotter with config. This is split out from Init() so that both Init() and reinitialize() may
// call it using a specific VolumeSnapshotter.
func (r *restartableVolumeSnapshotter) init(volumeSnapshotter velero.VolumeSnapshotter, config map[string]string) error {
	return volumeSnapshotter.Init(config)
}

// CreateVolumeFromSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) CreateVolumeFromSnapshot(snapshotID string, volumeType string, volumeAZ string, iops *int64) (volumeID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
}

// GetVolumeID restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) GetVolumeID(pv runtime.Unstructured) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.GetVolumeID(pv)
}

// SetVolumeID restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.SetVolumeID(pv, volumeID)
}

// GetVolumeInfo restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) GetVolumeInfo(volumeID string, volumeAZ string) (string, *int64, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", nil, err
	}
	return delegate.GetVolumeInfo(volumeID, volumeAZ)
}

// CreateSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) CreateSnapshot(volumeID string, volumeAZ string, tags map[string]string) (snapshotID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSnapshot(volumeID, volumeAZ, tags)
}

// DeleteSnapshot restarts the plugin's process if needed, then delegates the call.
func (r *restartableVolumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteSnapshot(snapshotID)
}
