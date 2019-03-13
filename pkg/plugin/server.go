/*
Copyright 2017, 2019 the Velero contributors.

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
	"fmt"
	"os"
	"strings"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/heptio/velero/pkg/util/logging"
)

// Handshake is configuration information that allows go-plugin clients and servers to perform a handshake.
//
// TODO(ncdc): this should probably be a function so it can't be mutated, and we should probably move it to
// handshake.go.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARK_PLUGIN",
	MagicCookieValue: "hello",
}

// Server serves registered plugin implementations.
type Server interface {
	// BindFlags defines the plugin server's command-line flags
	// on the provided FlagSet. If you're not sure what flag set
	// to use, pflag.CommandLine is the default set of command-line
	// flags.
	//
	// This method must be called prior to calling .Serve().
	BindFlags(flags *pflag.FlagSet) Server

	// RegisterBackupItemAction registers a backup item action.
	RegisterBackupItemAction(name string, initializer HandlerInitializer) Server

	// RegisterBackupItemActions registers multiple backup item actions.
	RegisterBackupItemActions(map[string]HandlerInitializer) Server

	// RegisterBlockStore registers a block store.
	RegisterBlockStore(name string, initializer HandlerInitializer) Server

	// RegisterBlockStores registers multiple block stores.
	RegisterBlockStores(map[string]HandlerInitializer) Server

	// RegisterObjectStore registers an object store.
	RegisterObjectStore(name string, initializer HandlerInitializer) Server

	// RegisterObjectStores registers multiple object stores.
	RegisterObjectStores(map[string]HandlerInitializer) Server

	// RegisterRestoreItemAction registers a restore item action.
	RegisterRestoreItemAction(name string, initializer HandlerInitializer) Server

	// RegisterRestoreItemActions registers multiple restore item actions.
	RegisterRestoreItemActions(map[string]HandlerInitializer) Server

	// Server runs the plugin server.
	Serve()
}

// server implements Server.
type server struct {
	log               *logrus.Logger
	logLevelFlag      *logging.LevelFlag
	flagSet           *pflag.FlagSet
	backupItemAction  *BackupItemActionPlugin
	blockStore        *BlockStorePlugin
	objectStore       *ObjectStorePlugin
	restoreItemAction *RestoreItemActionPlugin
}

// NewServer returns a new Server
func NewServer() Server {
	log := newLogger()

	return &server{
		log:               log,
		logLevelFlag:      logging.LogLevelFlag(log.Level),
		backupItemAction:  NewBackupItemActionPlugin(serverLogger(log)),
		blockStore:        NewBlockStorePlugin(serverLogger(log)),
		objectStore:       NewObjectStorePlugin(serverLogger(log)),
		restoreItemAction: NewRestoreItemActionPlugin(serverLogger(log)),
	}
}

func (s *server) BindFlags(flags *pflag.FlagSet) Server {
	flags.Var(s.logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(s.logLevelFlag.AllowedValues(), ", ")))
	s.flagSet = flags

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

func (s *server) RegisterBlockStore(name string, initializer HandlerInitializer) Server {
	s.blockStore.register(name, initializer)
	return s
}

func (s *server) RegisterBlockStores(m map[string]HandlerInitializer) Server {
	for name := range m {
		s.RegisterBlockStore(name, m[name])
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
		s.log.Infof("Parsing flags")
		s.flagSet.Parse(os.Args[1:])
	}

	s.log.Level = s.logLevelFlag.Parse()
	s.log.Infof("Setting log level to %s", strings.ToUpper(s.log.Level.String()))

	command := os.Args[0]

	var pluginIdentifiers []PluginIdentifier
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindBackupItemAction, s.backupItemAction)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindBlockStore, s.blockStore)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindObjectStore, s.objectStore)...)
	pluginIdentifiers = append(pluginIdentifiers, getNames(command, PluginKindRestoreItemAction, s.restoreItemAction)...)

	pluginLister := NewPluginLister(pluginIdentifiers...)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			string(PluginKindBackupItemAction):  s.backupItemAction,
			string(PluginKindBlockStore):        s.blockStore,
			string(PluginKindObjectStore):       s.objectStore,
			string(PluginKindPluginLister):      NewPluginListerPlugin(pluginLister),
			string(PluginKindRestoreItemAction): s.restoreItemAction,
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
