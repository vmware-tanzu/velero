/*
Copyright 2020 the Velero contributors.

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
	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

// RestoreItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the restore/ItemAction
// interface.
type DeleteItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

// GRPCClient returns a RestoreItemAction gRPC client.
func (p *DeleteItemActionPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, clientConn *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, clientConn, newDeleteItemActionGRPCClient), nil
}

// GRPCServer registers a DeleteItemAction gRPC server.
func (p *DeleteItemActionPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	proto.RegisterDeleteItemActionServer(server, &DeleteItemActionGRPCServer{mux: p.serverMux})
	return nil
}
