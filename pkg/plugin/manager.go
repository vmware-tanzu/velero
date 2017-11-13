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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/heptio/ark/pkg/cloudprovider"
)

// PluginKind is a type alias for a string that describes
// the kind of an Ark-supported plugin.
type PluginKind string

func (k PluginKind) String() string {
	return string(k)
}

const (
	// PluginKindObjectStore is the Kind string for
	// an Object Store plugin.
	PluginKindObjectStore PluginKind = "objectstore"

	// PluginKindBlockStore is the Kind string for
	// a Block Store plugin.
	PluginKindBlockStore PluginKind = "blockstore"

	pluginDir = "/plugins"
)

type pluginInfo struct {
	kind PluginKind
	name string
}

// Manager exposes functions for getting implementations of the pluggable
// Ark interfaces.
type Manager interface {
	// GetObjectStore returns the plugin implementation of the
	// cloudprovider.ObjectStore interface with the specified name.
	GetObjectStore(name string) (cloudprovider.ObjectStore, error)

	// GetBlockStore returns the plugin implementation of the
	// cloudprovider.BlockStore interface with the specified name.
	GetBlockStore(name string) (cloudprovider.BlockStore, error)
}

type manager struct {
	logger          hclog.Logger
	clients         map[pluginInfo]*plugin.Client
	internalPlugins map[pluginInfo]interface{}
}

// NewManager constructs a manager for getting plugin implementations.
func NewManager(logger logrus.FieldLogger, level logrus.Level) Manager {
	return &manager{
		logger:  (&logrusAdapter{impl: logger, level: level}),
		clients: make(map[pluginInfo]*plugin.Client),
		internalPlugins: map[pluginInfo]interface{}{
			{kind: PluginKindObjectStore, name: "aws"}: struct{}{},
			{kind: PluginKindBlockStore, name: "aws"}:  struct{}{},

			{kind: PluginKindObjectStore, name: "gcp"}: struct{}{},
			{kind: PluginKindBlockStore, name: "gcp"}:  struct{}{},

			{kind: PluginKindObjectStore, name: "azure"}: struct{}{},
			{kind: PluginKindBlockStore, name: "azure"}:  struct{}{},
		},
	}
}

func addPlugins(config *plugin.ClientConfig, kinds ...PluginKind) {
	for _, kind := range kinds {
		if kind == PluginKindObjectStore {
			config.Plugins[kind.String()] = &ObjectStorePlugin{}
		} else if kind == PluginKindBlockStore {
			config.Plugins[kind.String()] = &BlockStorePlugin{}
		}
	}
}

func (m *manager) getPlugin(descriptor pluginInfo, logger hclog.Logger) (interface{}, error) {
	client, found := m.clients[descriptor]
	if !found {
		var (
			externalPath = filepath.Join(pluginDir, fmt.Sprintf("ark-%s-%s", descriptor.kind, descriptor.name))
			config       = &plugin.ClientConfig{
				HandshakeConfig:  Handshake,
				AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
				Plugins:          make(map[string]plugin.Plugin),
				Logger:           logger,
			}
		)

		// First check to see if there's an external plugin for this kind and name. this
		// is so users can override the built-in plugins if they want. If it doesn't exist,
		// see if there's an internal one.
		if _, err := os.Stat(externalPath); err == nil {
			addPlugins(config, descriptor.kind)
			config.Cmd = exec.Command(externalPath)

			client = plugin.NewClient(config)

			m.clients[descriptor] = client
		} else if _, found := m.internalPlugins[descriptor]; found {
			addPlugins(config, PluginKindObjectStore, PluginKindBlockStore)
			config.Cmd = exec.Command("/ark", "plugin", "cloudprovider", descriptor.name)

			client = plugin.NewClient(config)

			// since a single sub-process will serve both an object and block store
			// for a given cloud-provider, record this client as being valid for both
			m.clients[pluginInfo{PluginKindObjectStore, descriptor.name}] = client
			m.clients[pluginInfo{PluginKindBlockStore, descriptor.name}] = client
		} else {
			return nil, errors.Errorf("plugin not found for kind=%s, name=%s", descriptor.kind, descriptor.name)
		}
	}

	protocolClient, err := client.Client()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	plugin, err := protocolClient.Dispense(descriptor.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return plugin, nil
}

// GetObjectStore returns the plugin implementation of the cloudprovider.ObjectStore
// interface with the specified name.
func (m *manager) GetObjectStore(name string) (cloudprovider.ObjectStore, error) {
	pluginObj, err := m.getPlugin(pluginInfo{PluginKindObjectStore, name}, m.logger)
	if err != nil {
		return nil, err
	}

	objStore, ok := pluginObj.(cloudprovider.ObjectStore)
	if !ok {
		return nil, errors.New("could not convert gRPC client to cloudprovider.ObjectStore")
	}

	return objStore, nil
}

// GetBlockStore returns the plugin implementation of the cloudprovider.BlockStore
// interface with the specified name.
func (m *manager) GetBlockStore(name string) (cloudprovider.BlockStore, error) {
	pluginObj, err := m.getPlugin(pluginInfo{PluginKindBlockStore, name}, m.logger)
	if err != nil {
		return nil, err
	}

	blockStore, ok := pluginObj.(cloudprovider.BlockStore)
	if !ok {
		return nil, errors.New("could not convert gRPC client to cloudprovider.BlockStore")
	}

	return blockStore, nil
}
