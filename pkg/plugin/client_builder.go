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
	logger      hclog.Logger
	commandName string
	commandArgs []string
}

// newClientBuilder returns a new clientBuilder with commandName to name. If the command matches the currently running
// process (i.e. ark), this also sets commandArgs to the internal Ark command to run plugins.
func newClientBuilder(command string, logger logrus.FieldLogger, logLevel logrus.Level) *clientBuilder {
	b := &clientBuilder{
		commandName: command,
		logger:      &logrusAdapter{impl: logger, level: logLevel},
	}
	if command == os.Args[0] {
		// For plugins compiled into the ark executable, we need to run "ark run-plugin"
		b.commandArgs = []string{"run-plugin"}
	}
	return b
}

// client creates a new go-plugin Client with support for all of Ark's plugin kinds (BackupItemAction, BlockStore,
// ObjectStore, PluginLister, RestoreItemAction).
func (b *clientBuilder) client() *hcplugin.Client {
	config := &hcplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		Plugins: map[string]hcplugin.Plugin{
			string(PluginKindBackupItemAction):  NewBackupItemActionPlugin(),
			string(PluginKindBlockStore):        NewBlockStorePlugin(),
			string(PluginKindObjectStore):       NewObjectStorePlugin(),
			string(PluginKindPluginLister):      &PluginListerPlugin{},
			string(PluginKindRestoreItemAction): NewRestoreItemActionPlugin(),
		},
		Logger: b.logger,
		Cmd:    exec.Command(b.commandName, b.commandArgs...),
	}
	return hcplugin.NewClient(config)
}
