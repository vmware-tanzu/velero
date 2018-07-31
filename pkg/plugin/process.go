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
	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ProcessFactory interface {
	newProcess(command string, logger logrus.FieldLogger, logLevel logrus.Level) (Process, error)
}

type processFactory struct {
}

func newProcessFactory() ProcessFactory {
	return &processFactory{}
}

func (pf *processFactory) newProcess(command string, logger logrus.FieldLogger, logLevel logrus.Level) (Process, error) {
	return newProcess(command, logger, logLevel)
}

type Process interface {
	dispense(key kindAndName) (interface{}, error)
	exited() bool
	kill()
}

type process struct {
	client         *plugin.Client
	protocolClient plugin.ClientProtocol
}

func newProcess(command string, logger logrus.FieldLogger, logLevel logrus.Level) (Process, error) {
	builder := newClientBuilder(command, logger.WithField("cmd", command), logLevel)

	// This creates a new go-plugin Client that has its own unique exec.Cmd for launching the plugin process.
	client := builder.client()

	// This launches the plugin process.
	protocolClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	p := &process{
		client:         client,
		protocolClient: protocolClient,
	}

	return p, nil
}

func (r *process) dispense(key kindAndName) (interface{}, error) {
	// This calls GRPCClient(clientConn) on the plugin instance registered for key.name.
	dispensed, err := r.protocolClient.Dispense(key.kind.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Currently all plugins except for PluginLister dispense clientDispenser instances.
	if clientDispenser, ok := dispensed.(ClientDispenser); ok {
		if key.name == "" {
			return nil, errors.Errorf("%s plugin requested but name is missing", key.kind.String())
		}
		// Get the instance that implements our plugin interface (e.g. cloudprovider.ObjectStore) that is a gRPC-based
		// client
		dispensed = clientDispenser.clientFor(key.name)
	}

	return dispensed, nil
}

func (r *process) exited() bool {
	return r.client.Exited()
}

func (r *process) kill() {
	r.client.Kill()
}
