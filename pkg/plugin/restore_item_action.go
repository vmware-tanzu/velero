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

package plugin

import (
	"encoding/json"

	"github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/heptio/ark/pkg/restore"
)

// RestoreItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the restore/ItemAction
// interface.
type RestoreItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl restore.ItemAction
	log  *logrusAdapter
}

// NewRestoreItemActionPlugin constructs a RestoreItemActionPlugin.
func NewRestoreItemActionPlugin(itemAction restore.ItemAction) *RestoreItemActionPlugin {
	return &RestoreItemActionPlugin{
		impl: itemAction,
	}
}

func (p *RestoreItemActionPlugin) Kind() PluginKind {
	return PluginKindRestoreItemAction
}

// GRPCServer registers a RestoreItemAction gRPC server.
func (p *RestoreItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterRestoreItemActionServer(s, &RestoreItemActionGRPCServer{impl: p.impl})
	return nil
}

// GRPCClient returns a RestoreItemAction gRPC client.
func (p *RestoreItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &RestoreItemActionGRPCClient{grpcClient: proto.NewRestoreItemActionClient(c), log: p.log}, nil
}

// RestoreItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type RestoreItemActionGRPCClient struct {
	grpcClient proto.RestoreItemActionClient
	log        *logrusAdapter
}

func (c *RestoreItemActionGRPCClient) AppliesTo() (restore.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.Empty{})
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

func (c *RestoreItemActionGRPCClient) Execute(item runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	itemJSON, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, err
	}

	restoreJSON, err := json.Marshal(restore)
	if err != nil {
		return nil, nil, err
	}

	req := &proto.RestoreExecuteRequest{
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

func (c *RestoreItemActionGRPCClient) SetLog(log logrus.FieldLogger) {
	c.log.impl = log
}

// RestoreItemActionGRPCServer implements the proto-generated RestoreItemActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type RestoreItemActionGRPCServer struct {
	impl restore.ItemAction
}

func (s *RestoreItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.Empty) (*proto.AppliesToResponse, error) {
	appliesTo, err := s.impl.AppliesTo()
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

	res, warning, err := s.impl.Execute(&item, &restore)
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
