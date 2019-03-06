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
package framework

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// clientBase implements client and contains shared fields common to all clients.
type ClientBase struct {
	Plugin string
	logger logrus.FieldLogger
}

type ClientDispenser interface {
	clientFor(name string) interface{}
}

// clientDispenser supports the initialization and retrieval of multiple implementations for a single plugin kind, such as
// "aws" and "azure" implementations of the object store plugin.
type clientDispenser struct {
	// Logger is the log the plugin should use.
	Logger logrus.FieldLogger
	// ClientConn is shared among all implementations for this client.
	ClientConn *grpc.ClientConn
	// InitFunc returns a client that implements a plugin interface, such as cloudprovider.ObjectStore.
	InitFunc clientInitFunc
	// Clients keeps track of all the initialized implementations.
	Clients map[string]interface{}
}

type clientInitFunc func(base *ClientBase, clientConn *grpc.ClientConn) interface{}

// newClientDispenser creates a new clientDispenser.
func NewClientDispenser(logger logrus.FieldLogger, clientConn *grpc.ClientConn, initFunc clientInitFunc) *clientDispenser {
	return &clientDispenser{
		ClientConn: clientConn,
		Logger:     logger,
		InitFunc:   initFunc,
		Clients:    make(map[string]interface{}),
	}
}

// clientFor returns a gRPC client stub for the implementation of a plugin named name. If the client stub does not
// currently exist, clientFor creates it.
func (cd *clientDispenser) clientFor(name string) interface{} {
	if client, found := cd.Clients[name]; found {
		return client
	}

	base := &ClientBase{
		Plugin: name,
		logger: cd.Logger,
	}
	// Initialize the plugin (e.g. newBackupItemActionGRPCClient())
	client := cd.InitFunc(base, cd.ClientConn)
	cd.Clients[name] = client

	return client
}
