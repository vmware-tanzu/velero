/*
Copyright 2021 the Velero contributors.

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
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	backupitemactionv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/backupitemaction/v2"
	deleteitemactionv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/deleteitemaction/v2"
	objectstorev2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v2"
	restoreitemactionv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/restoreitemaction/v2"
	volumesnapshotterv2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/volumesnapshotter/v2"
)

// Manager manages the lifecycles of plugins.
type Manager interface {
	// GetObjectStore returns the ObjectStore plugin for name.
	GetObjectStore(name string) (objectstorev2.ObjectStore, error)

	// GetVolumeSnapshotter returns the VolumeSnapshotter plugin for name.
	GetVolumeSnapshotter(name string) (volumesnapshotterv2.VolumeSnapshotter, error)

	// GetBackupItemActions returns all backup item action plugins.
	GetBackupItemActions() ([]backupitemactionv2.BackupItemAction, error)

	// GetBackupItemAction returns the backup item action plugin for name.
	GetBackupItemAction(name string) (backupitemactionv2.BackupItemAction, error)

	// GetRestoreItemActions returns all restore item action plugins.
	GetRestoreItemActions() ([]restoreitemactionv2.RestoreItemAction, error)

	// GetRestoreItemAction returns the restore item action plugin for name.
	GetRestoreItemAction(name string) (restoreitemactionv2.RestoreItemAction, error)

	// GetDeleteItemActions returns all delete item action plugins.
	GetDeleteItemActions() ([]deleteitemactionv2.DeleteItemAction, error)

	// GetDeleteItemAction returns the delete item action plugin for name.
	GetDeleteItemAction(name string) (deleteitemactionv2.DeleteItemAction, error)

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

type RestartableObjectStore struct {
	kind framework.PluginKind
	// Get returns a restartable ObjectStore for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) objectstorev2.ObjectStore
}

func (m *manager) restartableObjectStores() []RestartableObjectStore {
	return []RestartableObjectStore{
		{
			kind: framework.PluginKindObjectStoreV2,
			Get: func(name string, restartableProcess RestartableProcess) objectstorev2.ObjectStore {
				return newRestartableObjectStoreV2(name, restartableProcess)
			},
		},
		{
			kind: framework.PluginKindObjectStore,
			Get: func(name string, restartableProcess RestartableProcess) objectstorev2.ObjectStore {
				// Adapt v1 plugin to v2
				return newAdaptedV1ObjectStore(name, restartableProcess)
			},
		},
	}
}

// GetObjectStore returns a restartableObjectStore for name.
func (m *manager) GetObjectStore(name string) (objectstorev2.ObjectStore, error) {
	name = sanitizeName(name)
	for _, restartableObjStore := range m.restartableObjectStores() {
		restartableProcess, err := m.getRestartableProcess(restartableObjStore.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableObjStore.Get(name, restartableProcess), nil
	}
	return nil, fmt.Errorf("unable to get valid ObjectStore for %q", name)
}

type RestartableVolumeSnapshotter struct {
	kind framework.PluginKind
	// Get returns a restartable VolumeSnapshotter for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) volumesnapshotterv2.VolumeSnapshotter
}

func (m *manager) restartableVolumeSnapshotters() []RestartableVolumeSnapshotter {
	return []RestartableVolumeSnapshotter{
		{
			kind: framework.PluginKindVolumeSnapshotterV2,
			Get: func(name string, restartableProcess RestartableProcess) volumesnapshotterv2.VolumeSnapshotter {
				return newRestartableVolumeSnapshotterV2(name, restartableProcess)
			},
		},
		{
			kind: framework.PluginKindVolumeSnapshotter,
			Get: func(name string, restartableProcess RestartableProcess) volumesnapshotterv2.VolumeSnapshotter {
				// Adapt v1 plugin to v2
				return newAdaptedV1VolumeSnapshotter(name, restartableProcess)
			},
		},
	}
}

// GetVolumeSnapshotter returns a restartableVolumeSnapshotter for name.
func (m *manager) GetVolumeSnapshotter(name string) (volumesnapshotterv2.VolumeSnapshotter, error) {
	name = sanitizeName(name)
	for _, restartableVolumeSnapshotter := range m.restartableVolumeSnapshotters() {
		restartableProcess, err := m.getRestartableProcess(restartableVolumeSnapshotter.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableVolumeSnapshotter.Get(name, restartableProcess), nil
	}
	return nil, fmt.Errorf("unable to get valid VolumeSnapshotter for %q", name)
}

// GetBackupItemActions returns all backup item actions as restartableBackupItemActions.
func (m *manager) GetBackupItemActions() ([]backupitemactionv2.BackupItemAction, error) {
	list := m.registry.ListForKinds(framework.BackupItemActionKinds())
	actions := make([]backupitemactionv2.BackupItemAction, 0, len(list))

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

type RestartableBackupItemAction struct {
	kind framework.PluginKind
	// Get returns a restartable BackupItemAction for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) backupitemactionv2.BackupItemAction
}

func (m *manager) restartableBackupItemActions() []RestartableBackupItemAction {
	return []RestartableBackupItemAction{
		{
			kind: framework.PluginKindBackupItemActionV2,
			Get: func(name string, restartableProcess RestartableProcess) backupitemactionv2.BackupItemAction {
				return newRestartableBackupItemActionV2(name, restartableProcess)
			},
		},
		{
			kind: framework.PluginKindBackupItemAction,
			Get: func(name string, restartableProcess RestartableProcess) backupitemactionv2.BackupItemAction {
				// Adapt v1 plugin to v2
				return newAdaptedV1BackupItemAction(name, restartableProcess)
			},
		},
	}
}

// GetBackupItemAction returns a restartableBackupItemAction for name.
func (m *manager) GetBackupItemAction(name string) (backupitemactionv2.BackupItemAction, error) {
	name = sanitizeName(name)
	for _, restartableBackupItemAction := range m.restartableBackupItemActions() {
		restartableProcess, err := m.getRestartableProcess(restartableBackupItemAction.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableBackupItemAction.Get(name, restartableProcess), nil
	}
	return nil, fmt.Errorf("unable to get valid BackupItemAction for %q", name)
}

// GetRestoreItemActions returns all restore item actions as restartableRestoreItemActions.
func (m *manager) GetRestoreItemActions() ([]restoreitemactionv2.RestoreItemAction, error) {
	list := m.registry.ListForKinds(framework.RestoreItemActionKinds())

	actions := make([]restoreitemactionv2.RestoreItemAction, 0, len(list))

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

type RestartableRestoreItemAction struct {
	kind framework.PluginKind
	// Get returns a restartable RestoreItemAction for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) restoreitemactionv2.RestoreItemAction
}

func (m *manager) restartableRestoreItemActions() []RestartableRestoreItemAction {
	return []RestartableRestoreItemAction{
		{
			kind: framework.PluginKindRestoreItemActionV2,
			Get: func(name string, restartableProcess RestartableProcess) restoreitemactionv2.RestoreItemAction {
				return newRestartableRestoreItemActionV2(name, restartableProcess)
			},
		},
		{
			kind: framework.PluginKindRestoreItemAction,
			Get: func(name string, restartableProcess RestartableProcess) restoreitemactionv2.RestoreItemAction {
				// Adapt v1 plugin to v2
				return newAdaptedV1RestoreItemAction(name, restartableProcess)
			},
		},
	}
}

// GetRestoreItemAction returns a restartableRestoreItemAction for name.
func (m *manager) GetRestoreItemAction(name string) (restoreitemactionv2.RestoreItemAction, error) {
	name = sanitizeName(name)
	for _, restartableRestoreItemAction := range m.restartableRestoreItemActions() {
		restartableProcess, err := m.getRestartableProcess(restartableRestoreItemAction.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableRestoreItemAction.Get(name, restartableProcess), nil
	}
	return nil, fmt.Errorf("unable to get valid RestoreItemAction for %q", name)
}

// GetDeleteItemActions returns all delete item actions as restartableDeleteItemActions.
func (m *manager) GetDeleteItemActions() ([]deleteitemactionv2.DeleteItemAction, error) {
	list := m.registry.ListForKinds(framework.DeleteItemActionKinds())

	actions := make([]deleteitemactionv2.DeleteItemAction, 0, len(list))

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

type RestartableDeleteItemAction struct {
	kind framework.PluginKind
	// Get returns a restartable DeleteItemAction for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) deleteitemactionv2.DeleteItemAction
}

func (m *manager) restartableDeleteItemActions() []RestartableDeleteItemAction {
	return []RestartableDeleteItemAction{
		{
			kind: framework.PluginKindDeleteItemActionV2,
			Get: func(name string, restartableProcess RestartableProcess) deleteitemactionv2.DeleteItemAction {
				return newRestartableDeleteItemActionV2(name, restartableProcess)
			},
		},
		{
			kind: framework.PluginKindDeleteItemAction,
			Get: func(name string, restartableProcess RestartableProcess) deleteitemactionv2.DeleteItemAction {
				// Adapt v1 plugin to v2
				return newAdaptedV1DeleteItemAction(name, restartableProcess)
			},
		},
	}
}

// GetDeleteItemAction returns a restartableDeleteItemAction for name.
func (m *manager) GetDeleteItemAction(name string) (deleteitemactionv2.DeleteItemAction, error) {
	name = sanitizeName(name)
	for _, restartableDeleteItemAction := range m.restartableDeleteItemActions() {
		restartableProcess, err := m.getRestartableProcess(restartableDeleteItemAction.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableDeleteItemAction.Get(name, restartableProcess), nil
	}
	return nil, fmt.Errorf("unable to get valid DeleteItemAction for %q", name)
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
