package framework

import (
	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

type PreBackupActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

func (p *PreBackupActionPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, clientConn *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, clientConn, newPreBackupActionGRPCClient), nil
}

func (p *PreBackupActionPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	proto.RegisterPreBackupActionServer(server, &PreBackupActionGRPCServer{mux: p.serverMux})
	return nil
}
