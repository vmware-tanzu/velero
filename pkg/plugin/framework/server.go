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

package framework

import (
	"fmt"
	"os"
	"strings"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	veleroflag "github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

// Server serves registered plugin implementations.
type Server interface {
	// BindFlags defines the plugin server's command-line flags
	// on the provided FlagSet. If you're not sure what flag set
	// to use, pflag.CommandLine is the default set of command-line
	// flags.
	//
	// This method must be called prior to calling .Serve().
	BindFlags(flags *pflag.FlagSet) Server

	// RegisterBackupItemAction registers a backup item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterBackupItemAction(pluginName string, initializer HandlerInitializer) Server

	// RegisterBackupItemActions registers multiple backup item actions.
	RegisterBackupItemActions(map[string]HandlerInitializer) Server

	// RegisterVolumeSnapshotter registers a volume snapshotter. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterVolumeSnapshotter(pluginName string, initializer HandlerInitializer) Server

	// RegisterVolumeSnapshotters registers multiple volume snapshotters.
	RegisterVolumeSnapshotters(map[string]HandlerInitializer) Server

	// RegisterObjectStore registers an object store. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterObjectStore(pluginName string, initializer HandlerInitializer) Server

	// RegisterObjectStores registers multiple object stores.
	RegisterObjectStores(map[string]HandlerInitializer) Server

	// RegisterRestoreItemAction registers a restore item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterRestoreItemAction(pluginName string, initializer HandlerInitializer) Server

	// RegisterRestoreItemActions registers multiple restore item actions.
	RegisterRestoreItemActions(map[string]HandlerInitializer) Server

	// RegisterDeleteItemAction registers a delete item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterDeleteItemAction(pluginName string, initializer HandlerInitializer) Server

	// RegisterDeleteItemActions registers multiple Delete item actions.
	RegisterDeleteItemActions(map[string]HandlerInitializer) Server

	// Version 2

	// RegisterVolumeSnapshottersV2 registers multiple volume snapshotters.
	RegisterVolumeSnapshottersV2(map[string]HandlerInitializer) Server

	// RegisterObjectStoreV2 registers an object store. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterObjectStoreV2(pluginName string, initializer HandlerInitializer) Server

	// RegisterBackupItemActionV2 registers a backup item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterBackupItemActionV2(pluginName string, initializer HandlerInitializer) Server

	// RegisterBackupItemActionsV2 registers multiple backup item actions.
	RegisterBackupItemActionsV2(map[string]HandlerInitializer) Server

	// RegisterVolumeSnapshotterV2 registers a volume snapshotter. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterVolumeSnapshotterV2(pluginName string, initializer HandlerInitializer) Server

	// RegisterObjectStoresV2 registers multiple object stores.
	RegisterObjectStoresV2(map[string]HandlerInitializer) Server

	// RegisterRestoreItemActionV2 registers a restore item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterRestoreItemActionV2(pluginName string, initializer HandlerInitializer) Server

	// RegisterRestoreItemActionsV2 registers multiple restore item actions.
	RegisterRestoreItemActionsV2(map[string]HandlerInitializer) Server

	// RegisterDeleteItemActionV2 registers a delete item action. Accepted format
	// for the plugin name is <DNS subdomain>/<non-empty name>.
	RegisterDeleteItemActionV2(pluginName string, initializer HandlerInitializer) Server

	// RegisterDeleteItemActionsV2 registers multiple Delete item actions.
	RegisterDeleteItemActionsV2(map[string]HandlerInitializer) Server

	// Server runs the plugin server.
	Serve()
}

// server implements Server.
type server struct {
	log          *logrus.Logger
	logLevelFlag *logging.LevelFlag
	flagSet      *pflag.FlagSet
	featureSet   *veleroflag.StringArray
	// Version 1
	backupItemAction  *BackupItemActionPlugin
	volumeSnapshotter *VolumeSnapshotterPlugin
	objectStore       *ObjectStorePlugin
	restoreItemAction *RestoreItemActionPlugin
	deleteItemAction  *DeleteItemActionPlugin
	// Version 2
	backupItemActionV2  *BackupItemActionPlugin
	volumeSnapshotterV2 *VolumeSnapshotterPlugin
	objectStoreV2       *ObjectStorePlugin
	restoreItemActionV2 *RestoreItemActionPlugin
	deleteItemActionV2  *DeleteItemActionPlugin
}

// NewServer returns a new Server
func NewServer() Server {
	log := newLogger()
	features := veleroflag.NewStringArray()

	return &server{
		log:               log,
		logLevelFlag:      logging.LogLevelFlag(log.Level),
		featureSet:        &features,
		backupItemAction:  NewBackupItemActionPlugin(serverLogger(log)),
		volumeSnapshotter: NewVolumeSnapshotterPlugin(serverLogger(log)),
		objectStore:       NewObjectStorePlugin(serverLogger(log)),
		restoreItemAction: NewRestoreItemActionPlugin(serverLogger(log)),
		deleteItemAction:  NewDeleteItemActionPlugin(serverLogger(log)),
		backupItemActionV2:  NewBackupItemActionPlugin(serverLogger(log)),
		volumeSnapshotterV2: NewVolumeSnapshotterPlugin(serverLogger(log)),
		objectStoreV2:       NewObjectStorePlugin(serverLogger(log)),
		restoreItemActionV2: NewRestoreItemActionPlugin(serverLogger(log)),
		deleteItemActionV2:  NewDeleteItemActionPlugin(serverLogger(log)),
	}
}

func (s *server) BindFlags(flags *pflag.FlagSet) Server {
	flags.Var(s.logLevelFlag, "log-level", fmt.Sprintf("The level at which to log. Valid values are %s.", strings.Join(s.logLevelFlag.AllowedValues(), ", ")))
	flags.Var(s.featureSet, "features", "List of feature flags for this plugin")
	s.flagSet = flags
	s.flagSet.ParseErrorsWhitelist.UnknownFlags = true

	return s
}

func (s *server) RegisterBackupItemAction(name string, initializer HandlerInitializer) Server {
	s.backupItemAction.register(name, initializer)
	return s
}

func (s *server) RegisterBackupItemActions(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterBackupItemAction(name, m[name])
	}
	return s
}

func (s *server) RegisterVolumeSnapshotter(name string, initializer HandlerInitializer) Server {
	s.volumeSnapshotter.register(name, initializer)
	return s
}

func (s *server) RegisterVolumeSnapshotters(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterVolumeSnapshotter(name, m[name])
	}
	return s
}

func (s *server) RegisterObjectStore(name string, initializer HandlerInitializer) Server {
	s.objectStore.register(name, initializer)
	return s
}

func (s *server) RegisterObjectStores(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterObjectStore(name, m[name])
	}
	return s
}

func (s *server) RegisterRestoreItemAction(name string, initializer HandlerInitializer) Server {
	s.restoreItemAction.register(name, initializer)
	return s
}

func (s *server) RegisterRestoreItemActions(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterRestoreItemAction(name, m[name])
	}
	return s
}

func (s *server) RegisterDeleteItemAction(name string, initializer HandlerInitializer) Server {
	s.deleteItemAction.register(name, initializer)
	return s
}

func (s *server) RegisterDeleteItemActions(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterDeleteItemAction(name, m[name])
	}
	return s
}

// Version 2
func (s *server) RegisterBackupItemActionV2(name string, initializer HandlerInitializer) Server {
	s.backupItemActionV2.register(name, initializer)
	return s
}

func (s *server) RegisterBackupItemActionsV2(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterBackupItemActionV2(name, m[name])
	}
	return s
}

func (s *server) RegisterVolumeSnapshotterV2(name string, initializer HandlerInitializer) Server {
	s.volumeSnapshotterV2.register(name, initializer)
	return s
}

func (s *server) RegisterVolumeSnapshottersV2(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterVolumeSnapshotterV2(name, m[name])
	}
	return s
}

func (s *server) RegisterObjectStoreV2(name string, initializer HandlerInitializer) Server {
	s.objectStoreV2.register(name, initializer)
	return s
}

func (s *server) RegisterObjectStoresV2(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterObjectStoreV2(name, m[name])
	}
	return s
}

func (s *server) RegisterRestoreItemActionV2(name string, initializer HandlerInitializer) Server {
	s.restoreItemActionV2.register(name, initializer)
	return s
}

func (s *server) RegisterRestoreItemActionsV2(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterRestoreItemActionV2(name, m[name])
	}
	return s
}

func (s *server) RegisterDeleteItemActionV2(name string, initializer HandlerInitializer) Server {
	s.deleteItemActionV2.register(name, initializer)
	return s
}

func (s *server) RegisterDeleteItemActionsV2(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterDeleteItemActionV2(name, m[name])
	}
	return s
}

// getNames returns a list of PluginIdentifiers registered with plugin.
func getNames(command string, kind PluginKind, plugin Interface) []PluginIdentifier {
	var pluginIdentifiers []PluginIdentifier

	for _, name := range plugin.names() {
		id := PluginIdentifier{Command: command, Kind: kind, Name: name}
		pluginIdentifiers = append(pluginIdentifiers, id)
	}

	return pluginIdentifiers
}

func (s *server) Serve() {
	if s.flagSet != nil && !s.flagSet.Parsed() {
		s.log.Debugf("Parsing flags")
		s.flagSet.Parse(os.Args[1:])
	}

	s.log.Level = s.logLevelFlag.Parse()
	s.log.Debugf("Setting log level to %s", strings.ToUpper(s.log.Level.String()))

	command := os.Args[0]

	var pluginIdentifiers []PluginIdentifier
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindBackupItemAction, s.backupItemAction)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindVolumeSnapshotter, s.volumeSnapshotter)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindObjectStore, s.objectStore)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindRestoreItemAction, s.restoreItemAction)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindDeleteItemAction, s.deleteItemAction)...)
	// Version 2
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindBackupItemActionV2, s.backupItemActionV2)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindVolumeSnapshotterV2, s.volumeSnapshotterV2)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindObjectStoreV2, s.objectStoreV2)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindRestoreItemActionV2, s.restoreItemActionV2)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindDeleteItemActionV2, s.deleteItemActionV2)...)

	pluginLister := NewPluginLister(pluginIdentifiers...)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake(),
		Plugins: map[string]plugin.Plugin{
			string(PluginKindBackupItemAction):  s.backupItemAction,
			string(PluginKindVolumeSnapshotter): s.volumeSnapshotter,
			string(PluginKindObjectStore):       s.objectStore,
			string(PluginKindPluginLister):      NewPluginListerPlugin(pluginLister),
			string(PluginKindRestoreItemAction): s.restoreItemAction,
			string(PluginKindDeleteItemAction):  s.deleteItemAction,
			// Version 2
			string(PluginKindBackupItemActionV2):  s.backupItemActionV2,
			string(PluginKindVolumeSnapshotterV2): s.volumeSnapshotterV2,
			string(PluginKindObjectStoreV2):       s.objectStoreV2,
			string(PluginKindRestoreItemActionV2): s.restoreItemActionV2,
			string(PluginKindDeleteItemActionV2):  s.deleteItemActionV2,
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
