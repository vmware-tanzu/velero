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
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/heptio/ark/pkg/restore"
)

// RestoreItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the restore/ItemAction
// interface.
type RestoreItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	mux map[string]func() restore.ItemAction
	log *logrusAdapter
}

// NewRestoreItemActionPlugin constructs a RestoreItemActionPlugin.
func NewRestoreItemActionPlugin() *RestoreItemActionPlugin {
	return &RestoreItemActionPlugin{
		mux: make(map[string]func() restore.ItemAction),
	}
}

func (p *RestoreItemActionPlugin) Add(name string, f func() restore.ItemAction) *RestoreItemActionPlugin {
	p.mux[name] = f
	return p
}

func (p *RestoreItemActionPlugin) Names() []string {
	return sets.StringKeySet(p.mux).List()
}

// GRPCServer registers a RestoreItemAction gRPC server.
func (p *RestoreItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterRestoreItemActionServer(s, &RestoreItemActionGRPCServer{mux: p.mux, impls: make(map[string]restore.ItemAction)})
	return nil
}

// GRPCClient returns a RestoreItemAction gRPC client.
func (p *RestoreItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &restoreItemClientMux{
		grpcClient: proto.NewRestoreItemActionClient(c),
		log:        p.log,
		clients:    make(map[string]*RestoreItemActionGRPCClient),
	}, nil
}

// RestoreItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type RestoreItemActionGRPCClient struct {
	grpcClient proto.RestoreItemActionClient
	log        *logrusAdapter
	plugin     string
}

type restoreItemClientMux struct {
	grpcClient proto.RestoreItemActionClient
	log        *logrusAdapter
	clients    map[string]*RestoreItemActionGRPCClient
}

func (m *restoreItemClientMux) GetByName(name string) interface{} {
	if client, found := m.clients[name]; found {
		return client
	}
	client := &RestoreItemActionGRPCClient{
		plugin:     name,
		grpcClient: m.grpcClient,
		log:        m.log,
	}
	m.clients[name] = client
	return client
}

func (c *RestoreItemActionGRPCClient) AppliesTo() (restore.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.AppliesToRequest{Plugin: c.plugin})
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
		Plugin:  c.plugin,
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
	mux   map[string]func() restore.ItemAction
	impls map[string]restore.ItemAction
}

func (s *RestoreItemActionGRPCServer) getImpl(name string) restore.ItemAction {
	if impl, found := s.impls[name]; found {
		return impl
	}
	f := s.mux[name]
	s.impls[name] = f()
	return s.impls[name]
}

func (s *RestoreItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.AppliesToRequest) (*proto.AppliesToResponse, error) {
	impl := s.getImpl(req.Plugin)
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

	impl := s.getImpl(req.Plugin)
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
