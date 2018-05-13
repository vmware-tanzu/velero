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

// Manager exposes functions for getting implementations of the pluggable
// Ark interfaces.
type Manager interface {
	// GetObjectStore returns the plugin implementation of the
	// cloudprovider.ObjectStore interface with the specified name.
	GetObjectStore(name string) (cloudprovider.ObjectStore, error)

	// GetBlockStore returns the plugin implementation of the
	// cloudprovider.BlockStore interface with the specified name.
	GetBlockStore(name string) (cloudprovider.BlockStore, error)

	// GetBackupItemActions returns all backup.ItemAction plugins.
	// These plugin instances should ONLY be used for a single backup
	// (mainly because each one outputs to a per-backup log),
	// and should be terminated upon completion of the backup with
	// CloseBackupItemActions().
	GetBackupItemActions() ([]backup.ItemAction, error)

	// GetRestoreItemActions returns all restore.ItemAction plugins.
	// These plugin instances should ONLY be used for a single restore
	// (mainly because each one outputs to a per-restore log),
	// and should be terminated upon completion of the restore with
	// CloseRestoreItemActions().
	GetRestoreItemActions() ([]restore.ItemAction, error)

	// CleanupClients terminates all of the Manager's running plugin processes.
	CleanupClients()
}

// manager implements Manager.
type manager struct {
	logger   logrus.FieldLogger
	logLevel logrus.Level
	registry Registry

	// lock guards wrappers
	lock     sync.Mutex
	wrappers map[string]*wrapper
}

// NewManager constructs a manager for getting plugins.
func NewManager(logger logrus.FieldLogger, level logrus.Level, registry Registry) Manager {
	return &manager{
		logger:   logger,
		logLevel: level,
		registry: registry,
		wrappers: make(map[string]*wrapper),
	}
}

func (m *manager) CleanupClients() {
	m.lock.Lock()

	for _, wrapper := range m.wrappers {
		wrapper.stop()
	}

	m.lock.Unlock()
}

func (m *manager) getWrapper(kind PluginKind, name string) (*wrapper, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	logger := m.logger.WithFields(logrus.Fields{
		"kind": string(PluginKindObjectStore),
		"name": name,
	})
	logger.Debug("looking for plugin")

	info, err := m.registry.Get(kind, name)
	if err != nil {
		return nil, err
	}

	logger = logger.WithField("command", info.Command)

	wrapper, found := m.wrappers[info.Command]
	if found {
		logger.Debug("found preexisting plugin wrapper")
		return wrapper, nil
	}

	logger.Debug("creating new plugin wrapper")

	wrapper, err = newWrapper(info.Command, withLog(m.logger, m.logLevel))
	if err != nil {
		return nil, err
	}

	m.wrappers[info.Command] = wrapper

	return wrapper, nil
}

// GetObjectStore returns the plugin implementation of the cloudprovider.ObjectStore
// interface with the specified name.
func (m *manager) GetObjectStore(name string) (cloudprovider.ObjectStore, error) {
	wrapper, err := m.getWrapper(PluginKindObjectStore, name)
	if err != nil {
		return nil, err
	}

	r := newResumableObjectStore(name, wrapper)

	return r, nil
}

// GetBlockStore returns the plugin implementation of the cloudprovider.BlockStore
// interface with the specified name.
func (m *manager) GetBlockStore(name string) (cloudprovider.BlockStore, error) {
	wrapper, err := m.getWrapper(PluginKindObjectStore, name)
	if err != nil {
		return nil, err
	}

	r := newResumableBlockStore(name, wrapper)

	return r, nil
}

// GetBackupActions returns all backup.BackupAction plugins.
// These plugin instances should ONLY be used for a single backup
// (mainly because each one outputs to a per-backup log),
// and should be terminated upon completion of the backup with
// CloseBackupActions().
func (m *manager) GetBackupItemActions() ([]backup.ItemAction, error) {
	list := m.registry.List(PluginKindBackupItemAction)

	actions := make([]backup.ItemAction, 0, len(list))

	for i := range list {
		id := list[i]
		wrapper, err := m.getWrapper(PluginKindBackupItemAction, id.Name)
		if err != nil {
			return nil, err
		}

		r := newResumableBackupItemAction(id.Name, wrapper)
		actions = append(actions, r)
	}

	return actions, nil
}

func (m *manager) GetRestoreItemActions() ([]restore.ItemAction, error) {
	list := m.registry.List(PluginKindRestoreItemAction)

	actions := make([]restore.ItemAction, 0, len(list))

	for i := range list {
		id := list[i]
		wrapper, err := m.getWrapper(PluginKindRestoreItemAction, id.Name)
		if err != nil {
			return nil, err
		}

		r := newResumableRestoreItemAction(id.Name, wrapper)
		actions = append(actions, r)
	}

	return actions, nil
}
