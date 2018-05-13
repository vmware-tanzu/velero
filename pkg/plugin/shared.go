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
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/restore"
)

// Handshake is configuration information that allows go-plugin
// clients and servers to perform a handshake.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARK_PLUGIN",
	MagicCookieValue: "hello",
}

type SimpleServer interface {
	RegisterBackupItemActions(map[string]func() backup.ItemAction) SimpleServer
	RegisterBlockStores(map[string]func() cloudprovider.BlockStore) SimpleServer
	RegisterObjectStores(map[string]func() cloudprovider.ObjectStore) SimpleServer
	RegisterRestoreItemActions(map[string]func() restore.ItemAction) SimpleServer
	Serve()
}

type simpleServer struct {
	backupItemAction  *BackupItemActionPlugin
	blockStore        *BlockStorePlugin
	objectStore       *ObjectStorePlugin
	restoreItemAction *RestoreItemActionPlugin
}

func NewSimpleServer() SimpleServer {
	return &simpleServer{
		backupItemAction:  NewBackupItemActionPlugin(),
		blockStore:        NewBlockStorePlugin(),
		objectStore:       NewObjectStorePlugin(),
		restoreItemAction: NewRestoreItemActionPlugin(),
	}
}

func (s *simpleServer) RegisterBackupItemActions(m map[string]func() backup.ItemAction) SimpleServer {
	for name := range m {
		s.backupItemAction.Add(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterBlockStores(m map[string]func() cloudprovider.BlockStore) SimpleServer {
	for name := range m {
		s.blockStore.Add(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterObjectStores(m map[string]func() cloudprovider.ObjectStore) SimpleServer {
	for name := range m {
		s.objectStore.Add(name, m[name])
	}
	return s
}

func (s *simpleServer) RegisterRestoreItemActions(m map[string]func() restore.ItemAction) SimpleServer {
	for name := range m {
		s.restoreItemAction.Add(name, m[name])
	}
	return s
}

func getNames(command string, kind PluginKind, plugin Interface) []PluginIdentifier {
	var pluginIdentifiers []PluginIdentifier
	for _, name := range plugin.Names() {
		id := PluginIdentifier{Command: command, Kind: kind, Name: name}
		pluginIdentifiers = append(pluginIdentifiers, id)
	}
	return pluginIdentifiers
}

// Serve serves the plugin p.
func (s *simpleServer) Serve() {
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
