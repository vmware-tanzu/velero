/*
Copyright 2018, 2019 the Velero contributors.

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
	"os"
	"os/exec"
	"testing"

	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/heptio/velero/pkg/plugin/framework"
	"github.com/heptio/velero/pkg/util/test"
)

func TestNewClientBuilder(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("velero", logger, logLevel)
	assert.Equal(t, cb.commandName, "velero")
	assert.Equal(t, []string{"--log-level", "info"}, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)

	cb = newClientBuilder(os.Args[0], logger, logLevel)
	assert.Equal(t, cb.commandName, os.Args[0])
	assert.Equal(t, []string{"run-plugins", "--log-level", "info"}, cb.commandArgs)
	assert.Equal(t, newLogrusAdapter(logger, logLevel), cb.pluginLogger)
}

func TestClientConfig(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel
	cb := newClientBuilder("velero", logger, logLevel)

	expected := &hcplugin.ClientConfig{
		HandshakeConfig:  framework.Handshake(),
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		Plugins: map[string]hcplugin.Plugin{
			string(framework.PluginKindBackupItemAction):  framework.NewBackupItemActionPlugin(framework.ClientLogger(logger)),
			string(framework.PluginKindVolumeSnapshotter): framework.NewVolumeSnapshotterPlugin(framework.ClientLogger(logger)),
			string(framework.PluginKindObjectStore):       framework.NewObjectStorePlugin(framework.ClientLogger(logger)),
			string(framework.PluginKindPluginLister):      &framework.PluginListerPlugin{},
			string(framework.PluginKindRestoreItemAction): framework.NewRestoreItemActionPlugin(framework.ClientLogger(logger)),
		},
		Logger: cb.pluginLogger,
		Cmd:    exec.Command(cb.commandName, cb.commandArgs...),
	}

	cc := cb.clientConfig()
	assert.Equal(t, expected, cc)
}
