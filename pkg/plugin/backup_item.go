package plugin

import (
	"encoding/json"

	plugin "github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	proto "github.com/heptio/velero/pkg/plugin/generated"
	"github.com/heptio/velero/pkg/plugin/interface/actioninterface"
)

// BackupItemActionPlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the backup/ItemAction
// interface.
type BackupItemActionPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

// NewBackupItemActionPlugin constructs a BackupItemActionPlugin.
func NewBackupItemActionPlugin(options ...pluginOption) *BackupItemActionPlugin {
	return &BackupItemActionPlugin{
		pluginBase: newPluginBase(options...),
	}
}

// GRPCClient returns a clientDispenser for BackupItemAction gRPC clients.
func (p *BackupItemActionPlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, c, newBackupItemActionGRPCClient), nil
}

type BackupItemActionGRPCClient struct {
	*clientBase
	grpcClient proto.BackupItemActionClient
}

func newBackupItemActionGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &BackupItemActionGRPCClient{
		clientBase: base,
		grpcClient: proto.NewBackupItemActionClient(clientConn),
	}
}

func (c *BackupItemActionGRPCClient) AppliesTo() (actioninterface.ResourceSelector, error) {
	res, err := c.grpcClient.AppliesTo(context.Background(), &proto.AppliesToRequest{Plugin: c.plugin})
	if err != nil {
		return actioninterface.ResourceSelector{}, err
	}

	return actioninterface.ResourceSelector{
		IncludedNamespaces: res.IncludedNamespaces,
		ExcludedNamespaces: res.ExcludedNamespaces,
		IncludedResources:  res.IncludedResources,
		ExcludedResources:  res.ExcludedResources,
		LabelSelector:      res.Selector,
	}, nil
}

func (c *BackupItemActionGRPCClient) Execute(item runtime.Unstructured, bkp *api.Backup) (runtime.Unstructured, []actioninterface.ResourceIdentifier, error) {
	itemJSON, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, err
	}

	backupJSON, err := json.Marshal(bkp)
	if err != nil {
		return nil, nil, err
	}

	req := &proto.ExecuteRequest{
		Plugin: c.plugin,
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

	var additionalItems []actioninterface.ResourceIdentifier

	for _, itm := range res.AdditionalItems {
		newItem := actioninterface.ResourceIdentifier{
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
