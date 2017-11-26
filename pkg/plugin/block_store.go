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
	"github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/heptio/ark/pkg/cloudprovider"
	proto "github.com/heptio/ark/pkg/plugin/generated"
)

// BlockStorePlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the cloudprovider/BlockStore
// interface.
type BlockStorePlugin struct {
	plugin.NetRPCUnsupportedPlugin

	impl cloudprovider.BlockStore
}

// NewBlockStorePlugin constructs a BlockStorePlugin.
func NewBlockStorePlugin(blockStore cloudprovider.BlockStore) *BlockStorePlugin {
	return &BlockStorePlugin{
		impl: blockStore,
	}
}

// GRPCServer registers a BlockStore gRPC server.
func (p *BlockStorePlugin) GRPCServer(s *grpc.Server) error {
	proto.RegisterBlockStoreServer(s, &BlockStoreGRPCServer{impl: p.impl})
	return nil
}

// GRPCClient returns a BlockStore gRPC client.
func (p *BlockStorePlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return &BlockStoreGRPCClient{grpcClient: proto.NewBlockStoreClient(c)}, nil
}

// BlockStoreGRPCClient implements the cloudprovider.BlockStore interface and uses a
// gRPC client to make calls to the plugin server.
type BlockStoreGRPCClient struct {
	grpcClient proto.BlockStoreClient
}

// Init prepares the BlockStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the BlockStore
// cannot be initialized from the provided config.
func (c *BlockStoreGRPCClient) Init(config map[string]string) error {
	_, err := c.grpcClient.Init(context.Background(), &proto.InitRequest{Config: config})

	return err
}

// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
// and with the specified type and IOPS (if using provisioned IOPS).
func (c *BlockStoreGRPCClient) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	req := &proto.CreateVolumeRequest{
		SnapshotID: snapshotID,
		VolumeType: volumeType,
		VolumeAZ:   volumeAZ,
	}

	if iops == nil {
		req.Iops = 0
	} else {
		req.Iops = *iops
	}

	res, err := c.grpcClient.CreateVolumeFromSnapshot(context.Background(), req)
	if err != nil {
		return "", err
	}

	return res.VolumeID, nil
}

// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for a specified block
// volume.
func (c *BlockStoreGRPCClient) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	res, err := c.grpcClient.GetVolumeInfo(context.Background(), &proto.GetVolumeInfoRequest{VolumeID: volumeID, VolumeAZ: volumeAZ})
	if err != nil {
		return "", nil, err
	}

	var iops *int64
	if res.Iops != 0 {
		iops = &res.Iops
	}

	return res.VolumeType, iops, nil
}

// IsVolumeReady returns whether the specified volume is ready to be used.
func (c *BlockStoreGRPCClient) IsVolumeReady(volumeID, volumeAZ string) (bool, error) {
	res, err := c.grpcClient.IsVolumeReady(context.Background(), &proto.IsVolumeReadyRequest{VolumeID: volumeID, VolumeAZ: volumeAZ})
	if err != nil {
		return false, err
	}

	return res.Ready, nil
}

// ListSnapshots returns a list of all snapshots matching the specified set of tag key/values.
func (c *BlockStoreGRPCClient) ListSnapshots(tagFilters map[string]string) ([]string, error) {
	res, err := c.grpcClient.ListSnapshots(context.Background(), &proto.ListSnapshotsRequest{TagFilters: tagFilters})
	if err != nil {
		return nil, err
	}

	return res.SnapshotIDs, nil
}

// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
// set of tags to the snapshot.
func (c *BlockStoreGRPCClient) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	req := &proto.CreateSnapshotRequest{
		VolumeID: volumeID,
		VolumeAZ: volumeAZ,
		Tags:     tags,
	}

	res, err := c.grpcClient.CreateSnapshot(context.Background(), req)
	if err != nil {
		return "", err
	}

	return res.SnapshotID, nil
}

// DeleteSnapshot deletes the specified volume snapshot.
func (c *BlockStoreGRPCClient) DeleteSnapshot(snapshotID string) error {
	_, err := c.grpcClient.DeleteSnapshot(context.Background(), &proto.DeleteSnapshotRequest{SnapshotID: snapshotID})

	return err
}

// BlockStoreGRPCServer implements the proto-generated BlockStoreServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type BlockStoreGRPCServer struct {
	impl cloudprovider.BlockStore
}

// Init prepares the BlockStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the BlockStore
// cannot be initialized from the provided config.
func (s *BlockStoreGRPCServer) Init(ctx context.Context, req *proto.InitRequest) (*proto.Empty, error) {
	if err := s.impl.Init(req.Config); err != nil {
		return nil, err
	}

	return &proto.Empty{}, nil
}

// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
// and with the specified type and IOPS (if using provisioned IOPS).
func (s *BlockStoreGRPCServer) CreateVolumeFromSnapshot(ctx context.Context, req *proto.CreateVolumeRequest) (*proto.CreateVolumeResponse, error) {
	snapshotID := req.SnapshotID
	volumeType := req.VolumeType
	volumeAZ := req.VolumeAZ
	var iops *int64

	if req.Iops != 0 {
		iops = &req.Iops
	}

	volumeID, err := s.impl.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
	if err != nil {
		return nil, err
	}

	return &proto.CreateVolumeResponse{VolumeID: volumeID}, nil
}

// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for a specified block
// volume.
func (s *BlockStoreGRPCServer) GetVolumeInfo(ctx context.Context, req *proto.GetVolumeInfoRequest) (*proto.GetVolumeInfoResponse, error) {
	volumeType, iops, err := s.impl.GetVolumeInfo(req.VolumeID, req.VolumeAZ)
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

// IsVolumeReady returns whether the specified volume is ready to be used.
func (s *BlockStoreGRPCServer) IsVolumeReady(ctx context.Context, req *proto.IsVolumeReadyRequest) (*proto.IsVolumeReadyResponse, error) {
	ready, err := s.impl.IsVolumeReady(req.VolumeID, req.VolumeAZ)
	if err != nil {
		return nil, err
	}

	return &proto.IsVolumeReadyResponse{Ready: ready}, nil
}

// ListSnapshots returns a list of all snapshots matching the specified set of tag key/values.
func (s *BlockStoreGRPCServer) ListSnapshots(ctx context.Context, req *proto.ListSnapshotsRequest) (*proto.ListSnapshotsResponse, error) {
	snapshotIDs, err := s.impl.ListSnapshots(req.TagFilters)
	if err != nil {
		return nil, err
	}

	return &proto.ListSnapshotsResponse{SnapshotIDs: snapshotIDs}, nil
}

// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
// set of tags to the snapshot.
func (s *BlockStoreGRPCServer) CreateSnapshot(ctx context.Context, req *proto.CreateSnapshotRequest) (*proto.CreateSnapshotResponse, error) {
	snapshotID, err := s.impl.CreateSnapshot(req.VolumeID, req.VolumeAZ, req.Tags)
	if err != nil {
		return nil, err
	}

	return &proto.CreateSnapshotResponse{SnapshotID: snapshotID}, nil
}

// DeleteSnapshot deletes the specified volume snapshot.
func (s *BlockStoreGRPCServer) DeleteSnapshot(ctx context.Context, req *proto.DeleteSnapshotRequest) (*proto.Empty, error) {
	if err := s.impl.DeleteSnapshot(req.SnapshotID); err != nil {
		return nil, err
	}

	return &proto.Empty{}, nil
}
