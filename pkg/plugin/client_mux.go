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

// clientMux supports the initialization and retrieval of multiple implementations for a single plugin kind, such as
// "aws" and "azure" implementations of the object store plugin.
type clientMux struct {
	// clienConn is shared among all implementations for this client.
	clientConn *grpc.ClientConn
	// log is the log the plugin should use.
	log *logrusAdapter
	// initFunc returns a client that implements a plugin interface, such as cloudprovider.ObjectStore.
	initFunc func() client
	// clients keeps track of all the initialized implementations.
	clients map[string]client
}

// newClientMux creates a new clientMux.
func newClientMux(clientConn *grpc.ClientConn, initFunc func() client) *clientMux {
	m := &clientMux{
		clientConn: clientConn,
		initFunc:   initFunc,
		clients:    make(map[string]client),
	}

	return m
}

// clientFor returns a gRPC client stub for the implementation of a plugin named name. If the client stub does not
// currently exist, clientFor creates it.
func (m *clientMux) clientFor(name string) interface{} {
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
