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
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkbackup "github.com/heptio/ark/pkg/backup"
	proto "github.com/heptio/ark/pkg/plugin/generated"
)

// BackupItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the backup/ItemAction
// interface.
type BackupItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl arkbackup.ItemAction
	log  *logrusAdapter
}

// NewBackupItemActionPlugin constructs a BackupItemActionPlugin.
func NewBackupItemActionPlugin(itemAction arkbackup.ItemAction) *BackupItemActionPlugin {
	return &BackupItemActionPlugin{
		impl: itemAction,
	}
}

func (p *BackupItemActionPlugin) Kind() PluginKind {
	return PluginKindBackupItemAction
}

// GRPCServer registers a BackupItemAction gRPC server.
func (p *BackupItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBackupItemActionServer(s, &BackupItemActionGRPCServer{impl: p.impl})
	return nil
}

// GRPCClient returns a BackupItemAction gRPC client.
func (p *BackupItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &BackupItemActionGRPCClient{grpcClient: proto.NewBackupItemActionClient(c), log: p.log}, nil
}

// BackupItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type BackupItemActionGRPCClient struct {
	grpcClient proto.BackupItemActionClient
	log        *logrusAdapter
}

func (c *BackupItemActionGRPCClient) AppliesTo() (arkbackup.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.Empty{})
	if err != nil {
		return arkbackup.ResourceSelector{}, err
	}

	return arkbackup.ResourceSelector{
		IncludedNamespaces: res.IncludedNamespaces,
		ExcludedNamespaces: res.ExcludedNamespaces,
		IncludedResources:  res.IncludedResources,
		ExcludedResources:  res.ExcludedResources,
		LabelSelector:      res.Selector,
	}, nil
}

func (c *BackupItemActionGRPCClient) Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []arkbackup.ResourceIdentifier, error) {
	itemJSON, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, err
	}

	backupJSON, err := json.Marshal(backup)
	if err != nil {
		return nil, nil, err
	}

	req := &proto.ExecuteRequest{
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

	var additionalItems []arkbackup.ResourceIdentifier

	for _, itm := range res.AdditionalItems {
		newItem := arkbackup.ResourceIdentifier{
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

func (c *BackupItemActionGRPCClient) SetLog(log logrus.FieldLogger) {
	c.log.impl = log
}

// BackupItemActionGRPCServer implements the proto-generated BackupItemActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type BackupItemActionGRPCServer struct {
	impl arkbackup.ItemAction
}

func (s *BackupItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.Empty) (*proto.AppliesToResponse, error) {
	resourceSelector, err := s.impl.AppliesTo()
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

func (s *BackupItemActionGRPCServer) Execute(ctx context.Context, req *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
	var item unstructured.Unstructured
	var backup api.Backup

	if err := json.Unmarshal(req.Item, &item); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, err
	}

	updatedItem, additionalItems, err := s.impl.Execute(&item, &backup)
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

func backupResourceIdentifierToProto(id arkbackup.ResourceIdentifier) *proto.ResourceIdentifier {
	return &proto.ResourceIdentifier{
		Group:     id.Group,
		Resource:  id.Resource,
		Namespace: id.Namespace,
		Name:      id.Name,
	}
}
