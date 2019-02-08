package plugin

import (
	"encoding/json"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	proto "github.com/heptio/velero/pkg/plugin/generated"
	"github.com/heptio/velero/pkg/plugin/interface/volumeinterface"
)

// GRPCServer registers a BlockStore gRPC server.
func (p *BlockStorePlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBlockStoreServer(s, &BlockStoreGRPCServer{mux: p.serverMux})
	return nil
}

// BlockStoreGRPCServer implements the proto-generated BlockStoreServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type BlockStoreGRPCServer struct {
	mux *serverMux
}

func (s *BlockStoreGRPCServer) getImpl(name string) (volumeinterface.BlockStore, error) {
	impl, err := s.mux.getHandler(name)
	if err != nil {
		return nil, err
	}

	blockStore, ok := impl.(volumeinterface.BlockStore)
	if !ok {
		return nil, errors.Errorf("%T is not a block store", impl)
	}

	return blockStore, nil
}

// Init prepares the BlockStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the BlockStore
// cannot be initialized from the provided config.
func (s *BlockStoreGRPCServer) Init(ctx context.Context, req *proto.InitRequest) (*proto.Empty, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	if err := impl.Init(req.Config); err != nil {
		return nil, err
	}

	return &proto.Empty{}, nil
}

// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
// and with the specified type and IOPS (if using provisioned IOPS).
func (s *BlockStoreGRPCServer) CreateVolumeFromSnapshot(ctx context.Context, req *proto.CreateVolumeRequest) (*proto.CreateVolumeResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	snapshotID := req.SnapshotID
	volumeType := req.VolumeType
	volumeAZ := req.VolumeAZ
	var iops *int64

	if req.Iops != 0 {
		iops = &req.Iops
	}

	volumeID, err := impl.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
	if err != nil {
		return nil, err
	}

	return &proto.CreateVolumeResponse{VolumeID: volumeID}, nil
}

// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for a specified block
// volume.
func (s *BlockStoreGRPCServer) GetVolumeInfo(ctx context.Context, req *proto.GetVolumeInfoRequest) (*proto.GetVolumeInfoResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	volumeType, iops, err := impl.GetVolumeInfo(req.VolumeID, req.VolumeAZ)
	if err != nil {
		return nil, err
	}

	res := &proto.GetVolumeInfoResponse{
		VolumeType: volumeType,
	}

	if iops != nil {
		res.Iops = *iops
	}

	return res, nil
}

// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
// set of tags to the snapshot.
func (s *BlockStoreGRPCServer) CreateSnapshot(ctx context.Context, req *proto.CreateSnapshotRequest) (*proto.CreateSnapshotResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	snapshotID, err := impl.CreateSnapshot(req.VolumeID, req.VolumeAZ, req.Tags)
	if err != nil {
		return nil, err
	}

	return &proto.CreateSnapshotResponse{SnapshotID: snapshotID}, nil
}

// DeleteSnapshot deletes the specified volume snapshot.
func (s *BlockStoreGRPCServer) DeleteSnapshot(ctx context.Context, req *proto.DeleteSnapshotRequest) (*proto.Empty, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	if err := impl.DeleteSnapshot(req.SnapshotID); err != nil {
		return nil, err
	}

	return &proto.Empty{}, nil
}

func (s *BlockStoreGRPCServer) GetVolumeID(ctx context.Context, req *proto.GetVolumeIDRequest) (*proto.GetVolumeIDResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	var pv unstructured.Unstructured

	if err := json.Unmarshal(req.PersistentVolume, &pv); err != nil {
		return nil, err
	}

	volumeID, err := impl.GetVolumeID(&pv)
	if err != nil {
		return nil, err
	}

	return &proto.GetVolumeIDResponse{VolumeID: volumeID}, nil
}

func (s *BlockStoreGRPCServer) SetVolumeID(ctx context.Context, req *proto.SetVolumeIDRequest) (*proto.SetVolumeIDResponse, error) {
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, err
	}

	var pv unstructured.Unstructured

	if err := json.Unmarshal(req.PersistentVolume, &pv); err != nil {
		return nil, err
	}

	updatedPV, err := impl.SetVolumeID(&pv, req.VolumeID)
	if err != nil {
		return nil, err
	}

	updatedPVBytes, err := json.Marshal(updatedPV.UnstructuredContent())
	if err != nil {
		return nil, err
	}

	return &proto.SetVolumeIDResponse{PersistentVolume: updatedPVBytes}, nil
}
