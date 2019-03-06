/*
Copyright 2017 the Heptio Ark contributors.

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

package restore

import (
	"encoding/json"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/plugin/framework"
	proto "github.com/heptio/velero/pkg/plugin/generated"
	"github.com/heptio/velero/pkg/restore"
)

// ItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the restore/ItemAction
// interface.
type ItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*framework.PluginBase
}

// NewRestoreItemActionPlugin constructs a ItemActionPlugin for restores.
func NewRestoreItemActionPlugin(options ...framework.PluginOption) *ItemActionPlugin {
	return &ItemActionPlugin{
		PluginBase: framework.NewPluginBase(options...),
	}
}

//////////////////////////////////////////////////////////////////////////////
// client code
//////////////////////////////////////////////////////////////////////////////

// GRPCClient returns a ItemActionPlugin gRPC client for restore.
func (p *ItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return framework.NewClientDispenser(p.ClientLogger, c, newRestoreItemActionGRPCClient), nil
}

// ItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type ItemActionGRPCClient struct {
	*framework.ClientBase
	grpcClient proto.RestoreItemActionClient
}

func newRestoreItemActionGRPCClient(base *framework.ClientBase, clientConn *grpc.ClientConn) interface{} {
	return &ItemActionGRPCClient{
		ClientBase: base,
		grpcClient: proto.NewRestoreItemActionClient(clientConn),
	}
}

func (c *ItemActionGRPCClient) AppliesTo() (restore.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.AppliesToRequest{Plugin: c.Plugin})
	if err != nil {
		return restore.ResourceSelector{}, err
	}

	return restore.ResourceSelector{
		IncludedNamespaces: res.IncludedNamespaces,
		ExcludedNamespaces: res.ExcludedNamespaces,
		IncludedResources:  res.IncludedResources,
		ExcludedResources:  res.ExcludedResources,
		LabelSelector:      res.Selector,
	}, nil
}

func (c *ItemActionGRPCClient) Execute(item runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	itemJSON, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, err
	}

	restoreJSON, err := json.Marshal(restore)
	if err != nil {
		return nil, nil, err
	}

	req := &proto.RestoreExecuteRequest{
		Plugin:  c.Plugin,
		Item:    itemJSON,
		Restore: restoreJSON,
	}

	res, err := c.grpcClient.Execute(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	var updatedItem unstructured.Unstructured
	if err := json.Unmarshal(res.Item, &updatedItem); err != nil {
		return nil, nil, err
	}

	var warning error
	if res.Warning != "" {
		warning = errors.New(res.Warning)
	}

	return &updatedItem, warning, nil
}

//////////////////////////////////////////////////////////////////////////////
// server code
//////////////////////////////////////////////////////////////////////////////

// GRPCServer registers a RestoreItemAction gRPC server.
func (p *ItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterRestoreItemActionServer(s, &RestoreItemActionGRPCServer{mux: p.ServerMux})
	return nil
}

// RestoreItemActionGRPCServer implements the proto-generated RestoreItemActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type RestoreItemActionGRPCServer struct {
	mux *framework.ServerMux
}

func (s *RestoreItemActionGRPCServer) getImpl(name string) (restore.ItemAction, error) {
	impl, err := s.mux.GetHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(restore.ItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a restore item action", impl)
	}

	return itemAction, nil
}

func (s *RestoreItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.AppliesToRequest) (*proto.AppliesToResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	appliesTo, err := impl.AppliesTo()
	if err != nil {
		return nil, err
	}

	return &proto.AppliesToResponse{
		IncludedNamespaces: appliesTo.IncludedNamespaces,
		ExcludedNamespaces: appliesTo.ExcludedNamespaces,
		IncludedResources:  appliesTo.IncludedResources,
		ExcludedResources:  appliesTo.ExcludedResources,
		Selector:           appliesTo.LabelSelector,
	}, nil
}

func (s *RestoreItemActionGRPCServer) Execute(ctx context.Context, req *proto.RestoreExecuteRequest) (*proto.RestoreExecuteResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	var (
		item    unstructured.Unstructured
		restore api.Restore
	)

	if err := json.Unmarshal(req.Item, &item); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(req.Restore, &restore); err != nil {
		return nil, err
	}

	res, warning, err := impl.Execute(&item, &restore)
	if err != nil {
		return nil, err
	}

	updatedItem, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	var warnMessage string
	if warning != nil {
		warnMessage = warning.Error()
	}

	return &proto.RestoreExecuteResponse{
		Item:    updatedItem,
		Warning: warnMessage,
	}, nil
}
