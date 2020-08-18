/*
Copyright 2020 the Velero contributors.

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

package framework

import (
	"encoding/json"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

var _ velero.DeleteItemAction = &DeleteItemActionGRPCClient{}

// NewDeleteItemActionPlugin constructs a DeleteItemActionPlugin.
func NewDeleteItemActionPlugin(options ...PluginOption) *DeleteItemActionPlugin {
	return &DeleteItemActionPlugin{
		pluginBase: newPluginBase(options...),
	}
}

// DeleteItemActionGRPCClient implements the backup/ItemAction interface and uses a
// gRPC client to make calls to the plugin server.
type DeleteItemActionGRPCClient struct {
	*clientBase
	grpcClient proto.DeleteItemActionClient
}

func newDeleteItemActionGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &DeleteItemActionGRPCClient{
		clientBase: base,
		grpcClient: proto.NewDeleteItemActionClient(clientConn),
	}
}

func (c *DeleteItemActionGRPCClient) AppliesTo() (velero.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.DeleteItemActionAppliesToRequest{Plugin: c.plugin})
	if err != nil {
		return velero.ResourceSelector{}, fromGRPCError(err)
	}

	if res.ResourceSelector == nil {
		return velero.ResourceSelector{}, nil
	}

	return velero.ResourceSelector{
		IncludedNamespaces: res.ResourceSelector.IncludedNamespaces,
		ExcludedNamespaces: res.ResourceSelector.ExcludedNamespaces,
		IncludedResources:  res.ResourceSelector.IncludedResources,
		ExcludedResources:  res.ResourceSelector.ExcludedResources,
		LabelSelector:      res.ResourceSelector.Selector,
	}, nil
}

func (c *DeleteItemActionGRPCClient) Execute(input *velero.DeleteItemActionExecuteInput) error {
	itemJSON, err := json.Marshal(input.Item.UnstructuredContent())
	if err != nil {
		return errors.WithStack(err)
	}

	backupJSON, err := json.Marshal(input.Backup)
	if err != nil {
		return errors.WithStack(err)
	}

	req := &proto.DeleteItemActionExecuteRequest{
		Plugin: c.plugin,
		Item:   itemJSON,
		Backup: backupJSON,
	}

	// First return item is just an empty struct no matter what.
	if _, err = c.grpcClient.Execute(context.Background(), req); err != nil {
		return fromGRPCError(err)
	}

	return nil
}
