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

package framework

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// clientBase implements client and contains shared fields common to all clients.
type clientBase struct {
	plugin string
	logger logrus.FieldLogger
}

type ClientDispenser interface {
	ClientFor(name string) interface{}
}

// clientDispenser supports the initialization and retrieval of multiple implementations for a single plugin kind, such as
// "aws" and "azure" implementations of the object store plugin.
type clientDispenser struct {
	// logger is the log the plugin should use.
	logger logrus.FieldLogger
	// clienConn is shared among all implementations for this client.
	clientConn *grpc.ClientConn
	// initFunc returns a client that implements a plugin interface, such as ObjectStore.
	initFunc clientInitFunc
	// clients keeps track of all the initialized implementations.
	clients map[string]interface{}
}

type clientInitFunc func(base *clientBase, clientConn *grpc.ClientConn) interface{}

// newClientDispenser creates a new clientDispenser.
func newClientDispenser(logger logrus.FieldLogger, clientConn *grpc.ClientConn, initFunc clientInitFunc) *clientDispenser {
	return &clientDispenser{
		clientConn: clientConn,
		logger:     logger,
		initFunc:   initFunc,
		clients:    make(map[string]interface{}),
	}
}

// ClientFor returns a gRPC client stub for the implementation of a plugin named name. If the client stub does not
// currently exist, clientFor creates it.
func (cd *clientDispenser) ClientFor(name string) interface{} {
	if client, found := cd.clients[name]; found {
		return client
	}

	base := &clientBase{
		plugin: name,
		logger: cd.logger,
	}
	// Initialize the plugin (e.g. newBackupItemActionGRPCClient())
	client := cd.initFunc(base, cd.clientConn)
	cd.clients[name] = client

	return client
}
