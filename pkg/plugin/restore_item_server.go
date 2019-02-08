package plugin

import (
	"encoding/json"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	proto "github.com/heptio/velero/pkg/plugin/generated"
	"github.com/heptio/velero/pkg/plugin/interface/actioninterface"
)

// GRPCServer registers a RestoreItemAction gRPC server.
func (p *RestoreItemActionPlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterRestoreItemActionServer(s, &RestoreItemActionGRPCServer{mux: p.serverMux})
	return nil
}

// RestoreItemActionGRPCServer implements the proto-generated RestoreItemActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type RestoreItemActionGRPCServer struct {
	mux *serverMux
}

func (s *RestoreItemActionGRPCServer) getImpl(name string) (actioninterface.RestoreItemAction, error) {
	impl, err := s.mux.getHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(actioninterface.RestoreItemAction)
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
