package plugin

import (
	"os/exec"

	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
)

type clientBuilder struct {
	config *hcplugin.ClientConfig
}

func newClientBuilder(baseConfig *hcplugin.ClientConfig) *clientBuilder {
	return &clientBuilder{
		config: baseConfig,
	}
}

func (b *clientBuilder) withPlugin(kind PluginKind, plugin hcplugin.Plugin) *clientBuilder {
	if b.config.Plugins == nil {
		b.config.Plugins = make(map[string]hcplugin.Plugin)
	}
	b.config.Plugins[string(kind)] = plugin

	return b
}

func (b *clientBuilder) withLogger(logger hclog.Logger) *clientBuilder {
	b.config.Logger = logger

	return b
}

func (b *clientBuilder) withCommand(name string, args ...string) *clientBuilder {
	b.config.Cmd = exec.Command(name, args...)

	return b
}

func (b *clientBuilder) client() *hcplugin.Client {
	return hcplugin.NewClient(b.config)
}
