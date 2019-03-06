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

package backup

import (
	"encoding/json"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerobackup "github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/plugin/framework"
	proto "github.com/heptio/velero/pkg/plugin/generated"
)

// ItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the backup/ItemAction
// interface.
type ItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*framework.PluginBase
}

// NewBackupItemActionPlugin constructs a ItemActionPlugin for backups.
func NewBackupItemActionPlugin(options ...framework.PluginOption) *ItemActionPlugin {
	return &ItemActionPlugin{
		PluginBase: framework.NewPluginBase(options...),
	}
}

//////////////////////////////////////////////////////////////////////////////
// client code
//////////////////////////////////////////////////////////////////////////////

// GRPCClient returns a ItemActionPlugin gRPC client for backup.
func (p *ItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return framework.NewClientDispenser(p.ClientLogger, c, newBackupItemActionGRPCClient), nil
}

// ItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type ItemActionGRPCClient struct {
	*framework.ClientBase
	grpcClient proto.BackupItemActionClient
}

func newBackupItemActionGRPCClient(base *framework.ClientBase, clientConn *grpc.ClientConn) interface{} {
	return &ItemActionGRPCClient{
		ClientBase: base,
		grpcClient: proto.NewBackupItemActionClient(clientConn),
	}
}

func (c *ItemActionGRPCClient) AppliesTo() (velerobackup.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.AppliesToRequest{Plugin: c.Plugin})
	if err != nil {
		return velerobackup.ResourceSelector{}, err
	}

	return velerobackup.ResourceSelector{
		IncludedNamespaces: res.IncludedNamespaces,
		ExcludedNamespaces: res.ExcludedNamespaces,
		IncludedResources:  res.IncludedResources,
		ExcludedResources:  res.ExcludedResources,
		LabelSelector:      res.Selector,
	}, nil
}

func (c *ItemActionGRPCClient) Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []velerobackup.ResourceIdentifier, error) {
	itemJSON, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, err
	}

	backupJSON, err := json.Marshal(backup)
	if err != nil {
		return nil, nil, err
	}

	req := &proto.ExecuteRequest{
		Plugin: c.Plugin,
		Item:   itemJSON,
		Backup: backupJSON,
	}

	res, err := c.grpcClient.Execute(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	var updatedItem unstructured.Unstructured
	if err := json.Unmarshal(res.Item, &updatedItem); err != nil {
		return nil, nil, err
	}

	var additionalItems []velerobackup.ResourceIdentifier

	for _, itm := range res.AdditionalItems {
		newItem := velerobackup.ResourceIdentifier{
			GroupResource: schema.GroupResource{
				Group:    itm.Group,
				Resource: itm.Resource,
			},
			Namespace: itm.Namespace,
			Name:      itm.Name,
		}

		additionalItems = append(additionalItems, newItem)
	}

	return &updatedItem, additionalItems, nil
}

//////////////////////////////////////////////////////////////////////////////
// server code
//////////////////////////////////////////////////////////////////////////////

// GRPCServer registers a BackupItemAction gRPC server.
func (p *ItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBackupItemActionServer(s, &ItemActionGRPCServer{mux: p.ServerMux})
	return nil
}

// ItemActionGRPCServer implements the proto-generated ItemActionGRPCServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type ItemActionGRPCServer struct {
	mux *framework.ServerMux
}

func (s *ItemActionGRPCServer) getImpl(name string) (velerobackup.ItemAction, error) {
	impl, err := s.mux.GetHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(velerobackup.ItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a backup item action", impl)
	}

	return itemAction, nil
}

func (s *ItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.AppliesToRequest) (*proto.AppliesToResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	resourceSelector, err := impl.AppliesTo()
	if err != nil {
		return nil, err
	}

	return &proto.AppliesToResponse{
		IncludedNamespaces: resourceSelector.IncludedNamespaces,
		ExcludedNamespaces: resourceSelector.ExcludedNamespaces,
		IncludedResources:  resourceSelector.IncludedResources,
		ExcludedResources:  resourceSelector.ExcludedResources,
		Selector:           resourceSelector.LabelSelector,
	}, nil
}

func (s *ItemActionGRPCServer) Execute(ctx context.Context, req *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	var item unstructured.Unstructured
	var backup api.Backup

	if err := json.Unmarshal(req.Item, &item); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, err
	}

	updatedItem, additionalItems, err := impl.Execute(&item, &backup)
	if err != nil {
		return nil, err
	}

	// If the plugin implementation returned a nil updatedItem (meaning no modifications), reset updatedItem to the
	// original item.
	var updatedItemJSON []byte
	if updatedItem == nil {
		updatedItemJSON = req.Item
	} else {
		updatedItemJSON, err = json.Marshal(updatedItem.UnstructuredContent())
		if err != nil {
			return nil, err
		}
	}

	res := &proto.ExecuteResponse{
		Item: updatedItemJSON,
	}

	for _, item := range additionalItems {
		res.AdditionalItems = append(res.AdditionalItems, backupResourceIdentifierToProto(item))
	}

	return res, nil
}

func backupResourceIdentifierToProto(id velerobackup.ResourceIdentifier) *proto.ResourceIdentifier {
	return &proto.ResourceIdentifier{
		Group:     id.Group,
		Resource:  id.Resource,
		Namespace: id.Namespace,
		Name:      id.Name,
	}
}
