package plugin

import (
	"os/exec"

	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
)

type clientBuilder struct {
	logger      hclog.Logger
	commandName string
	commandArgs []string
}

func newClientBuilder() *clientBuilder {
	return &clientBuilder{}
}

func (b *clientBuilder) withLogger(logger hclog.Logger) *clientBuilder {
	b.logger = logger

	return b
}

func (b *clientBuilder) withCommand(name string, args ...string) *clientBuilder {
	// b.config.Cmd = exec.Command(name, args...)
	b.commandName = name
	b.commandArgs = args

	return b
}

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
