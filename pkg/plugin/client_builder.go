package plugin

import (
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
)

// clientBuilder builds go-plugin Clients.
type clientBuilder struct {
	logger      hclog.Logger
	commandName string
	commandArgs []string
}

// newClientBuilder returns a new clientBuilder.
func newClientBuilder() *clientBuilder {
	// TODO(ncdc): make this take in command and logger and remove withLogger() and withCommand()
	return &clientBuilder{}
}

// withLogger sets the clientBuilder's logger to logger.
func (b *clientBuilder) withLogger(logger hclog.Logger) *clientBuilder {
	b.logger = logger

	return b
}

// withCommand sets the clientBuilder's commandName to name and commandArgs to args.
func (b *clientBuilder) withCommand(name string) *clientBuilder {
	b.commandName = name

	if name == os.Args[0] {
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
