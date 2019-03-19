/*
Copyright 2019 the Velero contributors.

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
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	proto "github.com/heptio/velero/pkg/plugin/generated"
)

// BlockStorePlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the cloudprovider/BlockStore
// interface.
type BlockStorePlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

// GRPCClient returns a BlockStore gRPC client.
func (p *BlockStorePlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, c, newBlockStoreGRPCClient), nil
}

// GRPCServer registers a BlockStore gRPC server.
func (p *BlockStorePlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBlockStoreServer(s, &BlockStoreGRPCServer{mux: p.serverMux})
	return nil
}
