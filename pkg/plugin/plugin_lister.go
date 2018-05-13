package plugin

import (
	plugin "github.com/hashicorp/go-plugin"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type PluginIdentifier struct {
	Command string
	Kind    PluginKind
	Name    string
}

type PluginLister interface {
	ListPlugins() ([]PluginIdentifier, error)
}

type pluginLister struct {
	plugins []PluginIdentifier
}

func NewPluginLister(plugins ...PluginIdentifier) PluginLister {
	return &pluginLister{plugins: plugins}
}

func (pl *pluginLister) ListPlugins() ([]PluginIdentifier, error) {
	return pl.plugins, nil
}

type PluginListerPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl PluginLister
}

func NewPluginListerPlugin(impl PluginLister) *PluginListerPlugin {
	return &PluginListerPlugin{impl: impl}
}

func (p *PluginListerPlugin) Kind() PluginKind {
	return PluginKindPluginLister
}

func (p *PluginListerPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterPluginListerServer(s, &PluginListerGRPCServer{impl: p.impl})
	return nil
}

// GRPCClient returns an ObjectStore gRPC client.
func (p *PluginListerPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &PluginListerGRPCClient{grpcClient: proto.NewPluginListerClient(c)}, nil
}

type PluginListerGRPCClient struct {
	grpcClient proto.PluginListerClient
}

func (c *PluginListerGRPCClient) ListPlugins() ([]PluginIdentifier, error) {
	resp, err := c.grpcClient.ListPlugins(context.Background(), &proto.Empty{})
	if err != nil {
		return nil, err
	}

	ret := make([]PluginIdentifier, len(resp.Plugins))
	for i, id := range resp.Plugins {
		if !AllPluginKinds.Has(id.Kind) {
			return nil, errors.Errorf("invalid plugin kind: %s", id.Kind)
		}

		ret[i] = PluginIdentifier{
			Command: id.Command,
			Kind:    PluginKind(id.Kind),
			Name:    id.Name,
		}
	}

	return ret, nil
}

type PluginListerGRPCServer struct {
	impl PluginLister
}

func (s *PluginListerGRPCServer) ListPlugins(ctx context.Context, req *proto.Empty) (*proto.ListPluginsResponse, error) {
	list, err := s.impl.ListPlugins()
	if err != nil {
		return nil, err
	}

	plugins := make([]*proto.PluginIdentifier, len(list))
	for i, id := range list {
		if !AllPluginKinds.Has(id.Kind.String()) {
			return nil, errors.Errorf("invalid plugin kind: %s", id.Kind)
		}

		plugins[i] = &proto.PluginIdentifier{
			Command: id.Command,
			Kind:    id.Kind.String(),
			Name:    id.Name,
		}
	}
	ret := &proto.ListPluginsResponse{
		Plugins: plugins,
	}
	return ret, nil
}
