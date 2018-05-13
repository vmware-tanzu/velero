package plugin

import (
	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type process struct {
	client         *plugin.Client
	protocolClient plugin.ClientProtocol
}

func newProcess(command string, logger logrus.FieldLogger, logLevel logrus.Level) (*process, error) {
	builder := newClientBuilder(command, logger.WithField("cmd", command), logLevel)

	// This creates a new go-plugin Client that has its own unique exec.Cmd for launching the plugin process.
	client := builder.client()

	// This launches the plugin process.
	protocolClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	runner := &process{
		client:         client,
		protocolClient: protocolClient,
	}

	return runner, nil
}

func (r *process) dispense(key kindAndName) (interface{}, error) {
	// This calls GRPCClient(clientConn) on the plugin instance registered for key.name.
	dispensed, err := r.protocolClient.Dispense(key.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Currently all plugins except for PluginLister dispense clientMux instances.
	if mux, ok := dispensed.(*clientMux); ok {
		if key.name == "" {
			return nil, errors.Errorf("%s plugin requested but name is missing", key.kind.String())
		}
		// Get the instance that implements our plugin interface (e.g. cloudprovider.ObjectStore) that is a gRPC-based
		// client
		dispensed = mux.clientFor(key.name)
	}

	return dispensed, nil
}

func (r *process) exited() bool {
	return r.client.Exited()
}

func (r *process) kill() {
	r.client.Kill()
}
