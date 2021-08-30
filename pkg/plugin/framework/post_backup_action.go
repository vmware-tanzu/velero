package framework

import (
	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

type PostBackupActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

func (p *PostBackupActionPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, clientConn *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, clientConn, newPostBackupActionGRPCClient), nil
}

func (p *PostBackupActionPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	proto.RegisterPostBackupActionServer(server, &PostBackupActionGRPCServer{mux: p.serverMux})
	return nil
}
