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
	"os"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
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
	backupItemAction  *BackupItemActionPlugin
	blockStore        *BlockStorePlugin
	objectStore       *ObjectStorePlugin
	restoreItemAction *RestoreItemActionPlugin
}

// NewServer returns a new Server
func NewServer(log logrus.FieldLogger) Server {
	return &server{
		backupItemAction:  NewBackupItemActionPlugin(serverLogger(log)),
		blockStore:        NewBlockStorePlugin(serverLogger(log)),
		objectStore:       NewObjectStorePlugin(serverLogger(log)),
		restoreItemAction: NewRestoreItemActionPlugin(serverLogger(log)),
	}
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
