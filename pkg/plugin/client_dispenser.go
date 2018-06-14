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

import "google.golang.org/grpc"

// client represents a gRPC-based client instance.
type client interface {
	// setPlugin sets the name of the plugin implementation, for example "aws" for a BlockStore plugin.
	setPlugin(name string)
	// setGrpcClientConn allows the client to create a new gRPC client stub using clientConn.
	setGrpcClientConn(clientConn *grpc.ClientConn)
	// setLog sets the log the client should use.
	setLog(log *logrusAdapter)
}

// clientBase implements client and contains shared fields common to all clients.
type clientBase struct {
	plugin string
	log    *logrusAdapter
}

func (c *clientBase) setPlugin(name string) {
	c.plugin = name
}

func (c *clientBase) setLog(log *logrusAdapter) {
	c.log = log
}

// clientDispenser supports the initialization and retrieval of multiple implementations for a single plugin kind, such as
// "aws" and "azure" implementations of the object store plugin.
type clientDispenser struct {
	// clienConn is shared among all implementations for this client.
	clientConn *grpc.ClientConn
	// log is the log the plugin should use.
	log *logrusAdapter
	// initFunc returns a client that implements a plugin interface, such as cloudprovider.ObjectStore.
	initFunc func() client
	// clients keeps track of all the initialized implementations.
	clients map[string]client
}

// newClientDispenser creates a new clientDispenser.
func newClientDispenser(clientConn *grpc.ClientConn, initFunc func() client) *clientDispenser {
	m := &clientDispenser{
		clientConn: clientConn,
		initFunc:   initFunc,
		clients:    make(map[string]client),
	}

	return m
}

// clientFor returns a gRPC client stub for the implementation of a plugin named name. If the client stub does not
// currently exist, clientFor creates it.
func (m *clientDispenser) clientFor(name string) interface{} {
	if client, found := m.clients[name]; found {
		return client
	}

	// Initialize the plugin (e.g. newBackupItemActionGRPCClient())
	client := m.initFunc()

	// Set the implementation name (e.g. "pod")
	client.setPlugin(name)
	// Set the gRPC clientConn, so it can create the stub (e.g. proto.NewBackupItemActionClient(clientConn))
	client.setGrpcClientConn(m.clientConn)
	// Set the log that the XYZGRPCClient can use
	client.setLog(m.log)
	m.clients[name] = client
	return client
}
