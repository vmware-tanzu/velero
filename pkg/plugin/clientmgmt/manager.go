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
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// Manager manages the lifecycles of plugins.
type Manager interface {
	// GetObjectStore returns the ObjectStore plugin for name.
	GetObjectStore(name string) (velero.ObjectStore, error)

	// GetVolumeSnapshotter returns the VolumeSnapshotter plugin for name.
	GetVolumeSnapshotter(name string) (velero.VolumeSnapshotter, error)

	// GetBackupItemActions returns all backup item action plugins.
	GetBackupItemActions() ([]velero.BackupItemAction, error)

	// GetBackupItemAction returns the backup item action plugin for name.
	GetBackupItemAction(name string) (velero.BackupItemAction, error)

	// GetRestoreItemActions returns all restore item action plugins.
	GetRestoreItemActions() ([]velero.RestoreItemAction, error)

	// GetRestoreItemAction returns the restore item action plugin for name.
	GetRestoreItemAction(name string) (velero.RestoreItemAction, error)

	// GetDeleteItemActions returns all delete item action plugins.
	GetDeleteItemActions() ([]velero.DeleteItemAction, error)

	// GetDeleteItemAction returns the delete item action plugin for name.
	GetDeleteItemAction(name string) (velero.DeleteItemAction, error)

	// CleanupClients terminates all of the Manager's running plugin processes.
	CleanupClients()
}

// manager implements Manager.
type manager struct {
	logger   logrus.FieldLogger
	logLevel logrus.Level
	registry Registry

	restartableProcessFactory RestartableProcessFactory

	// lock guards restartableProcesses
	lock                 sync.Mutex
	restartableProcesses map[string]RestartableProcess
}

// NewManager constructs a manager for getting plugins.
func NewManager(logger logrus.FieldLogger, level logrus.Level, registry Registry) Manager {
	return &manager{
		logger:   logger,
		logLevel: level,
		registry: registry,

		restartableProcessFactory: newRestartableProcessFactory(),

		restartableProcesses: make(map[string]RestartableProcess),
	}
}

func (m *manager) CleanupClients() {
	m.lock.Lock()

	for _, restartableProcess := range m.restartableProcesses {
		restartableProcess.stop()
	}

	m.lock.Unlock()
}

// getRestartableProcess returns a restartableProcess for a plugin identified by kind and name, creating a
// restartableProcess if it is the first time it has been requested.
func (m *manager) getRestartableProcess(kind framework.PluginKind, name string) (RestartableProcess, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	logger := m.logger.WithFields(logrus.Fields{
		"kind": framework.PluginKindObjectStore.String(),
		"name": name,
	})
	logger.Debug("looking for plugin in registry")

	info, err := m.registry.Get(kind, name)
	if err != nil {
		return nil, err
	}

	logger = logger.WithField("command", info.Command)

	restartableProcess, found := m.restartableProcesses[info.Command]
	if found {
		logger.Debug("found preexisting restartable plugin process")
		return restartableProcess, nil
	}

	logger.Debug("creating new restartable plugin process")

	restartableProcess, err = m.restartableProcessFactory.newRestartableProcess(info.Command, m.logger, m.logLevel)
	if err != nil {
		return nil, err
	}

	m.restartableProcesses[info.Command] = restartableProcess

	return restartableProcess, nil
}

// GetObjectStore returns a restartableObjectStore for name.
func (m *manager) GetObjectStore(name string) (velero.ObjectStore, error) {
	name = sanitizeName(name)

	restartableProcess, err := m.getRestartableProcess(framework.PluginKindObjectStore, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableObjectStore(name, restartableProcess)

	return r, nil
}

// GetVolumeSnapshotter returns a restartableVolumeSnapshotter for name.
func (m *manager) GetVolumeSnapshotter(name string) (velero.VolumeSnapshotter, error) {
	name = sanitizeName(name)

	restartableProcess, err := m.getRestartableProcess(framework.PluginKindVolumeSnapshotter, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableVolumeSnapshotter(name, restartableProcess)

	return r, nil
}

// GetBackupItemActions returns all backup item actions as restartableBackupItemActions.
func (m *manager) GetBackupItemActions() ([]velero.BackupItemAction, error) {
	list := m.registry.List(framework.PluginKindBackupItemAction)

	actions := make([]velero.BackupItemAction, 0, len(list))

	for i := range list {
		id := list[i]

		r, err := m.GetBackupItemAction(id.Name)
		if err != nil {
			return nil, err
		}

		actions = append(actions, r)
	}

	return actions, nil
}

// GetBackupItemAction returns a restartableBackupItemAction for name.
func (m *manager) GetBackupItemAction(name string) (velero.BackupItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, err := m.getRestartableProcess(framework.PluginKindBackupItemAction, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableBackupItemAction(name, restartableProcess)
	return r, nil
}

// GetRestoreItemActions returns all restore item actions as restartableRestoreItemActions.
func (m *manager) GetRestoreItemActions() ([]velero.RestoreItemAction, error) {
	list := m.registry.List(framework.PluginKindRestoreItemAction)

	actions := make([]velero.RestoreItemAction, 0, len(list))

	for i := range list {
		id := list[i]

		r, err := m.GetRestoreItemAction(id.Name)
		if err != nil {
			return nil, err
		}

		actions = append(actions, r)
	}

	return actions, nil
}

// GetRestoreItemAction returns a restartableRestoreItemAction for name.
func (m *manager) GetRestoreItemAction(name string) (velero.RestoreItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, err := m.getRestartableProcess(framework.PluginKindRestoreItemAction, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableRestoreItemAction(name, restartableProcess)
	return r, nil
}

// GetDeleteItemActions returns all delete item actions as restartableDeleteItemActions.
func (m *manager) GetDeleteItemActions() ([]velero.DeleteItemAction, error) {
	list := m.registry.List(framework.PluginKindDeleteItemAction)

	actions := make([]velero.DeleteItemAction, 0, len(list))

	for i := range list {
		id := list[i]

		r, err := m.GetDeleteItemAction(id.Name)
		if err != nil {
			return nil, err
		}

		actions = append(actions, r)
	}

	return actions, nil
}

// GetDeleteItemAction returns a restartableDeleteItemAction for name.
func (m *manager) GetDeleteItemAction(name string) (velero.DeleteItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, err := m.getRestartableProcess(framework.PluginKindDeleteItemAction, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableDeleteItemAction(name, restartableProcess)
	return r, nil
}

// sanitizeName adds "velero.io" to legacy plugins that weren't namespaced.
func sanitizeName(name string) string {
	// Backwards compatibility with non-namespaced Velero plugins, following principle of least surprise
	// since DeleteItemActions were not bundled with Velero when plugins were non-namespaced.
	if !strings.Contains(name, "/") {
		name = "velero.io/" + name
	}

	return name
}
