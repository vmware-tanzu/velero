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

// Handshake is configuration information that allows go-plugin
// clients and servers to perform a handshake.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARK_PLUGIN",
	MagicCookieValue: "hello",
}

type SimpleServer interface {
	RegisterBackupItemAction(name string, factory ServerImplFactory) SimpleServer
	RegisterBackupItemActions(map[string]ServerImplFactory) SimpleServer

	RegisterBlockStore(name string, factory ServerImplFactory) SimpleServer
	RegisterBlockStores(map[string]ServerImplFactory) SimpleServer

	RegisterObjectStore(name string, factory ServerImplFactory) SimpleServer
	RegisterObjectStores(map[string]ServerImplFactory) SimpleServer

	RegisterRestoreItemAction(name string, factory ServerImplFactory) SimpleServer
	RegisterRestoreItemActions(map[string]ServerImplFactory) SimpleServer

	Serve()
}

type simpleServer struct {
	log               logrus.FieldLogger
	backupItemAction  *BackupItemActionPlugin
	blockStore        *BlockStorePlugin
	objectStore       *ObjectStorePlugin
	restoreItemAction *RestoreItemActionPlugin
}

func NewSimpleServer(log logrus.FieldLogger) SimpleServer {
	backupItemAction := NewBackupItemActionPlugin()
	backupItemAction.setServerLog(log)

	blockStore := NewBlockStorePlugin()
	blockStore.setServerLog(log)

	objectStore := NewObjectStorePlugin()
	objectStore.setServerLog(log)

	restoreItemAction := NewRestoreItemActionPlugin()
	restoreItemAction.setServerLog(log)

	return &simpleServer{
		log:               log,
		backupItemAction:  backupItemAction,
		blockStore:        blockStore,
		objectStore:       objectStore,
		restoreItemAction: restoreItemAction,
	}
}

func (s *simpleServer) RegisterBackupItemAction(name string, factory ServerImplFactory) SimpleServer {
	s.backupItemAction.register(name, factory)
	return s
}

func (s *simpleServer) RegisterBackupItemActions(m map[string]ServerImplFactory) SimpleServer {
	for name := range m {
		s.RegisterBackupItemAction(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterBlockStore(name string, factory ServerImplFactory) SimpleServer {
	s.blockStore.register(name, factory)
	return s
}

func (s *simpleServer) RegisterBlockStores(m map[string]ServerImplFactory) SimpleServer {
	for name := range m {
		s.RegisterBlockStore(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterObjectStore(name string, factory ServerImplFactory) SimpleServer {
	s.objectStore.register(name, factory)
	return s
}

func (s *simpleServer) RegisterObjectStores(m map[string]ServerImplFactory) SimpleServer {
	for name := range m {
		s.RegisterObjectStore(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterRestoreItemAction(name string, factory ServerImplFactory) SimpleServer {
	s.restoreItemAction.register(name, factory)
	return s
}

func (s *simpleServer) RegisterRestoreItemActions(m map[string]ServerImplFactory) SimpleServer {
	for name := range m {
		s.RegisterRestoreItemAction(name, m[name])
	}
	return s
}

func getNames(command string, kind PluginKind, plugin Interface) []PluginIdentifier {
	var pluginIdentifiers []PluginIdentifier
	for _, name := range plugin.names() {
		id := PluginIdentifier{Command: command, Kind: kind, Name: name}
		pluginIdentifiers = append(pluginIdentifiers, id)
	}
	return pluginIdentifiers
}

// Serve serves the plugin p.
func (s *simpleServer) Serve() {
	defer func() {
		s.log.Error("ANDY ANDY ANDY")
	}()

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
