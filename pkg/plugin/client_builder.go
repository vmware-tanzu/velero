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

	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
)

// clientBuilder builds go-plugin Clients.
type clientBuilder struct {
	commandName  string
	commandArgs  []string
	clientLogger logrus.FieldLogger
	pluginLogger hclog.Logger
}

// newClientBuilder returns a new clientBuilder with commandName to name. If the command matches the currently running
// process (i.e. ark), this also sets commandArgs to the internal Ark command to run plugins.
func newClientBuilder(command string, logger logrus.FieldLogger, logLevel logrus.Level) *clientBuilder {
	b := &clientBuilder{
		commandName:  command,
		clientLogger: logger,
		pluginLogger: newLogrusAdapter(logger, logLevel),
	}
	if command == os.Args[0] {
		// For plugins compiled into the ark executable, we need to run "ark run-plugins"
		b.commandArgs = []string{"run-plugins"}
	}
	return b
}

func newLogrusAdapter(pluginLogger logrus.FieldLogger, logLevel logrus.Level) *logrusAdapter {
	return &logrusAdapter{impl: pluginLogger, level: logLevel}
}

func (b *clientBuilder) clientConfig() *hcplugin.ClientConfig {
	return &hcplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		Plugins: map[string]hcplugin.Plugin{
			string(PluginKindBackupItemAction):  NewBackupItemActionPlugin(clientLogger(b.clientLogger)),
			string(PluginKindBlockStore):        NewBlockStorePlugin(clientLogger(b.clientLogger)),
			string(PluginKindObjectStore):       NewObjectStorePlugin(clientLogger(b.clientLogger)),
			string(PluginKindPluginLister):      &PluginListerPlugin{},
			string(PluginKindRestoreItemAction): NewRestoreItemActionPlugin(clientLogger(b.clientLogger)),
		},
		Logger: b.pluginLogger,
		Cmd:    exec.Command(b.commandName, b.commandArgs...),
	}
}

// client creates a new go-plugin Client with support for all of Ark's plugin kinds (BackupItemAction, BlockStore,
// ObjectStore, PluginLister, RestoreItemAction).
func (b *clientBuilder) client() *hcplugin.Client {
	return hcplugin.NewClient(b.clientConfig())
}
