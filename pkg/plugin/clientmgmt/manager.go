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

func (m *manager) getRestartableProcessOfKinds(
	kinds []framework.PluginKind, name string) (RestartableProcess, framework.PluginKind, error) {
	var err error
	var process RestartableProcess
	var kind framework.PluginKind
	for _, kind = range kinds {
		process, err = m.getRestartableProcess(kind, name)
		if err == nil {
			return process, kind, nil
		} else if !errors.Is(err, &pluginNotFoundError{}) {
			return nil, kind, err
		}
	}
	return nil, kind, err
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

type NewObjectStoreFunction func(string, RestartableProcess) objectstorev2.ObjectStore

var ObjectStoreFunctions = map[framework.PluginKind]NewObjectStoreFunction{
	framework.PluginKindObjectStoreV2: newRestartableObjectStoreV2,
	framework.PluginKindObjectStore:   newAdaptedV1ObjectStore,
}

// GetObjectStore returns a restartableObjectStore for name.
func (m *manager) GetObjectStore(name string) (objectstorev2.ObjectStore, error) {
	name = sanitizeName(name)

	restartableProcess, kind, err := m.getRestartableProcessOfKinds(framework.ObjectStoreKinds(), name)
	if err != nil {
		return nil, err
	}

	f, ok := ObjectStoreFunctions[kind]
	if !ok || f == nil {
		err = fmt.Errorf("Unable to create ObjectStore for kind '%s'.", kind)
		return nil, err
	}
	return f(name, restartableProcess), nil
}

type NewVolumeSnapshotterFunction func(string, RestartableProcess) volumesnapshotterv2.VolumeSnapshotter

var VolumeSnapshotterFunctions = map[framework.PluginKind]NewVolumeSnapshotterFunction{
	framework.PluginKindVolumeSnapshotterV2: newRestartableVolumeSnapshotterV2,
	framework.PluginKindVolumeSnapshotter:   newAdaptedV1VolumeSnapshotter,
}

// GetVolumeSnapshotter returns a restartableVolumeSnapshotter for name.
func (m *manager) GetVolumeSnapshotter(name string) (volumesnapshotterv2.VolumeSnapshotter, error) {
	name = sanitizeName(name)

	restartableProcess, kind, err := m.getRestartableProcessOfKinds(framework.VolumeSnapshotterKinds(), name)
	if err != nil {
		return nil, err
	}

	f, ok := VolumeSnapshotterFunctions[kind]
	if !ok || f == nil {
		err = fmt.Errorf("Unable to create VolumeSnapshotter for kind '%s'.", kind)
		return nil, err
	}
	return f(name, restartableProcess), nil
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

type NewBackupItemActionFunction func(string, RestartableProcess) backupitemactionv2.BackupItemAction

var BackupItemActionFunctions = map[framework.PluginKind]NewBackupItemActionFunction {
	framework.PluginKindBackupItemActionV2: newRestartableBackupItemActionV2,
	framework.PluginKindBackupItemAction:   newAdaptedV1BackupItemAction,
}

// GetBackupItemAction returns a restartableBackupItemAction for name.
func (m *manager) GetBackupItemAction(name string) (backupitemactionv2.BackupItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, kind, err := m.getRestartableProcessOfKinds(framework.BackupItemActionKinds(), name)
	if err != nil {
		return nil, err
	}

	f, ok := BackupItemActionFunctions[kind]
	if !ok || f == nil {
		err = fmt.Errorf("Unable to create BackupItemAction for kind '%s'.", kind)
		return nil, err
	}
	return f(name, restartableProcess), nil
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

type NewRestoreItemActionFunction func(string, RestartableProcess) restoreitemactionv2.RestoreItemAction

var RestoreItemActionFunctions = map[framework.PluginKind]NewRestoreItemActionFunction {
	framework.PluginKindRestoreItemActionV2: newRestartableRestoreItemActionV2,
	framework.PluginKindRestoreItemAction:   newAdaptedV1RestoreItemAction,
}

// GetRestoreItemAction returns a restartableRestoreItemAction for name.
func (m *manager) GetRestoreItemAction(name string) (restoreitemactionv2.RestoreItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, kind, err := m.getRestartableProcessOfKinds(framework.RestoreItemActionKinds(), name)
	if err != nil {
		return nil, err
	}

	f, ok := RestoreItemActionFunctions[kind]
	if !ok || f == nil {
		err = fmt.Errorf("Unable to create RestoreItemAction for kind '%s'.", kind)
		return nil, err
	}
	return f(name, restartableProcess), nil
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

type NewDeleteItemActionFunction func(string, RestartableProcess) deleteitemactionv2.DeleteItemAction
var DeleteItemActionFunctions = map[framework.PluginKind]NewDeleteItemActionFunction {
	framework.PluginKindDeleteItemActionV2: newRestartableDeleteItemActionV2,
	framework.PluginKindDeleteItemAction:   newAdaptedV1DeleteItemAction,
}

// GetDeleteItemAction returns a restartableDeleteItemAction for name.
func (m *manager) GetDeleteItemAction(name string) (deleteitemactionv2.DeleteItemAction, error) {
	name = sanitizeName(name)

	restartableProcess, kind, err := m.getRestartableProcessOfKinds(framework.DeleteItemActionKinds(), name)
	if err != nil {
		return nil, err
	}

	f, ok := DeleteItemActionFunctions[kind]
	if !ok || f == nil {
		err = fmt.Errorf("Unable to create DeleteItemAction for kind '%s'.", kind)
		return nil, err
	}
	return f(name, restartableProcess), nil
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
