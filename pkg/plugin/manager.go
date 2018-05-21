/*
Copyright 2017 the Heptio Ark contributors.

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
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/restore"
)

// Manager manages the lifecycles of plugins.
type Manager interface {
	// GetObjectStore returns the ObjectStore plugin for name.
	GetObjectStore(name string) (cloudprovider.ObjectStore, error)

	// GetBlockStore returns the BlockStore plugin for name.
	GetBlockStore(name string) (cloudprovider.BlockStore, error)

	// GetBackupItemActions returns all backup item action plugins.
	GetBackupItemActions() ([]backup.ItemAction, error)

	// GetBackupItemAction returns the backup item action plugin for name.
	GetBackupItemAction(name string) (backup.ItemAction, error)

	// GetRestoreItemActions returns all restore item action plugins.
	GetRestoreItemActions() ([]restore.ItemAction, error)

	// GetRestoreItemAction returns the restore item action plugin for name.
	GetRestoreItemAction(name string) (restore.ItemAction, error)

	// CleanupClients terminates all of the Manager's running plugin processes.
	CleanupClients()
}

// manager implements Manager.
type manager struct {
	logger   logrus.FieldLogger
	logLevel logrus.Level
	registry Registry

	// lock guards restartablePluginProcesses
	lock                       sync.Mutex
	restartablePluginProcesses map[string]*restartablePluginProcess
}

// NewManager constructs a manager for getting plugins.
func NewManager(logger logrus.FieldLogger, level logrus.Level, registry Registry) Manager {
	return &manager{
		logger:                     logger,
		logLevel:                   level,
		registry:                   registry,
		restartablePluginProcesses: make(map[string]*restartablePluginProcess),
	}
}

func (m *manager) CleanupClients() {
	m.lock.Lock()

	for _, wrapper := range m.restartablePluginProcesses {
		wrapper.stop()
	}

	m.lock.Unlock()
}

// getWrapper returns a wrapper for a plugin identified by kind and name, creating a wrapper if it is the first time it
// has been requested.
func (m *manager) getWrapper(kind PluginKind, name string) (*restartablePluginProcess, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	logger := m.logger.WithFields(logrus.Fields{
		"kind": PluginKindObjectStore.String(),
		"name": name,
	})
	logger.Debug("looking for plugin in registry")

	info, err := m.registry.Get(kind, name)
	if err != nil {
		return nil, err
	}

	logger = logger.WithField("command", info.Command)

	wrapper, found := m.restartablePluginProcesses[info.Command]
	if found {
		logger.Debug("found preexisting plugin wrapper")
		return wrapper, nil
	}

	logger.Debug("creating new plugin wrapper")

	wrapper, err = newRestartablePluginProcess(info.Command, withLog(m.logger, m.logLevel))
	if err != nil {
		return nil, err
	}

	m.restartablePluginProcesses[info.Command] = wrapper

	return wrapper, nil
}

// GetObjectStore returns a resumableObjectStore for name.
func (m *manager) GetObjectStore(name string) (cloudprovider.ObjectStore, error) {
	wrapper, err := m.getWrapper(PluginKindObjectStore, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableObjectStore(name, wrapper)

	return r, nil
}

// GetBlockStore returns a resumableBlockStore for name.
func (m *manager) GetBlockStore(name string) (cloudprovider.BlockStore, error) {
	wrapper, err := m.getWrapper(PluginKindObjectStore, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableBlockStore(name, wrapper)

	return r, nil
}

// GetBackupItemActions returns all backup item actions as resumableBackupItemActions.
func (m *manager) GetBackupItemActions() ([]backup.ItemAction, error) {
	list := m.registry.List(PluginKindBackupItemAction)

	actions := make([]backup.ItemAction, 0, len(list))

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

// GetBackupItemAction returns a resumableBackupItemAction for name.
func (m *manager) GetBackupItemAction(name string) (backup.ItemAction, error) {
	wrapper, err := m.getWrapper(PluginKindBackupItemAction, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableBackupItemAction(name, wrapper)
	return r, nil
}

// GetRestoreItemActions returns all restore item actions as restartableRestoreItemActions.
func (m *manager) GetRestoreItemActions() ([]restore.ItemAction, error) {
	list := m.registry.List(PluginKindRestoreItemAction)

	actions := make([]restore.ItemAction, 0, len(list))

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
func (m *manager) GetRestoreItemAction(name string) (restore.ItemAction, error) {
	wrapper, err := m.getWrapper(PluginKindRestoreItemAction, name)
	if err != nil {
		return nil, err
	}

	r := newRestartableRestoreItemAction(name, wrapper)
	return r, nil
}
