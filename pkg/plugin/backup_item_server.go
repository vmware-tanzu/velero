package plugin

import (
	"encoding/json"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	proto "github.com/heptio/ark/pkg/plugin/generated"
	"github.com/heptio/ark/pkg/plugin/interface/actioninterface"
)

// GRPCServer registers a BackupItemAction gRPC server.
func (p *BackupItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBackupItemActionServer(s, &BackupItemActionGRPCServer{mux: p.serverMux})
	return nil
}

// BackupItemActionGRPCServer implements the proto-generated BackupItemActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type BackupItemActionGRPCServer struct {
	mux *serverMux
}

func (s *BackupItemActionGRPCServer) getImpl(name string) (actioninterface.BackupItemAction, error) {
	impl, err := s.mux.getHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(actioninterface.BackupItemAction)
	if !ok {
		return nil, errors.Errorf("%T is not a backup item action", impl)
	}

	return itemAction, nil
}

func (s *BackupItemActionGRPCServer) AppliesTo(ctx context.Context, req *proto.AppliesToRequest) (*proto.AppliesToResponse, error) {
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

func (s *BackupItemActionGRPCServer) Execute(ctx context.Context, req *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
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

func backupResourceIdentifierToProto(id actioninterface.ResourceIdentifier) *proto.ResourceIdentifier {
	return &proto.ResourceIdentifier{
		Group:     id.Group,
		Resource:  id.Resource,
		Namespace: id.Namespace,
		Name:      id.Name,
	}
}
