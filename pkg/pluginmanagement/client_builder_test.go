/*
Copyright 2018 the Heptio Ark contributors.

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
package pluginmanagement

import (
	"os"
	"os/exec"
	"testing"

	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/heptio/velero/pkg/plugin"
	"github.com/heptio/velero/pkg/util/test"
)

func TestNewClientBuilder(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("velero", logger, logLevel)
	assert.Equal(t, cb.commandName, "velero")
	assert.Empty(t, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)

	cb = newClientBuilder(os.Args[0], logger, logLevel)
	assert.Equal(t, cb.commandName, os.Args[0])
	assert.Equal(t, []string{"run-plugins"}, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)
}

func TestClientConfig(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("velero", logger, logLevel)

	expected := &hcplugin.ClientConfig{
		HandshakeConfig:  plugin.Handshake,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		Plugins: map[string]hcplugin.Plugin{
			string(plugin.PluginKindBackupItemAction):  plugin.NewBackupItemActionPlugin(plugin.ClientLogger(logger)),
			string(plugin.PluginKindBlockStore):        plugin.NewBlockStorePlugin(plugin.ClientLogger(logger)),
			string(plugin.PluginKindObjectStore):       plugin.NewObjectStorePlugin(plugin.ClientLogger(logger)),
			string(plugin.PluginKindPluginLister):      &plugin.PluginListerPlugin{},
			string(plugin.PluginKindRestoreItemAction): plugin.NewRestoreItemActionPlugin(plugin.ClientLogger(logger)),
		},
		Logger: cb.pluginLogger,
		Cmd:    exec.Command(cb.commandName, cb.commandArgs...),
	}

	cc := cb.clientConfig()
	assert.Equal(t, expected, cc)
}
