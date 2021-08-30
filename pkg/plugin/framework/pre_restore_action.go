package framework

import (
	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

type PreRestoreActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

func (p *PreRestoreActionPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, clientConn *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, clientConn, newPreRestoreActionGRPCClient), nil
}

func (p *PreRestoreActionPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	proto.RegisterPreRestoreActionServer(server, &PreRestoreActionGRPCServer{mux: p.serverMux})
	return nil
}
