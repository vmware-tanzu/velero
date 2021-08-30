package framework

import (
	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

type PostRestoreActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

func (p *PostRestoreActionPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, clientConn *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, clientConn, newPostRestoreActionGRPCClient), nil
}

func (p *PostRestoreActionPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	proto.RegisterPostRestoreActionServer(server, &PostRestoreActionGRPCServer{mux: p.serverMux})
	return nil
}
