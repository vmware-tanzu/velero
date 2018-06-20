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
package plugin

import (
	"os"
	"os/exec"
	"testing"

	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/heptio/ark/pkg/util/test"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewClientBuilder(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("ark", logger, logLevel)
	assert.Equal(t, cb.commandName, "ark")
	assert.Empty(t, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)

	cb = newClientBuilder(os.Args[0], logger, logLevel)
	assert.Equal(t, cb.commandName, os.Args[0])
	assert.Equal(t, []string{"run-plugin"}, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)
}

func TestClientConfig(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("ark", logger, logLevel)

	expected := &hcplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		Plugins: map[string]hcplugin.Plugin{
			string(PluginKindBackupItemAction):  NewBackupItemActionPlugin(clientLogger(logger)),
			string(PluginKindBlockStore):        NewBlockStorePlugin(clientLogger(logger)),
			string(PluginKindObjectStore):       NewObjectStorePlugin(clientLogger(logger)),
			string(PluginKindPluginLister):      &PluginListerPlugin{},
			string(PluginKindRestoreItemAction): NewRestoreItemActionPlugin(clientLogger(logger)),
		},
		Logger: cb.pluginLogger,
		Cmd:    exec.Command(cb.commandName, cb.commandArgs...),
	}

	cc := cb.clientConfig()
	assert.Equal(t, expected, cc)
}
