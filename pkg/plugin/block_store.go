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
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/ark/pkg/cloudprovider"
	proto "github.com/heptio/ark/pkg/plugin/generated"
)

// BlockStorePlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the cloudprovider/BlockStore
// interface.
type BlockStorePlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

// NewBlockStorePlugin constructs a BlockStorePlugin.
func NewBlockStorePlugin(options ...pluginOption) *BlockStorePlugin {
	return &BlockStorePlugin{
		pluginBase: newPluginBase(options...),
	}
}

//////////////////////////////////////////////////////////////////////////////
// client code
//////////////////////////////////////////////////////////////////////////////

// GRPCClient returns a BlockStore gRPC client.
func (p *BlockStorePlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, c, newBlockStoreGRPCClient), nil
}

// BlockStoreGRPCClient implements the cloudprovider.BlockStore interface and uses a
// gRPC client to make calls to the plugin server.
type BlockStoreGRPCClient struct {
	*clientBase
	grpcClient proto.BlockStoreClient
}

func newBlockStoreGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &BlockStoreGRPCClient{
		clientBase: base,
		grpcClient: proto.NewBlockStoreClient(clientConn),
	}
}

// Init prepares the BlockStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the BlockStore
// cannot be initialized from the provided config.
func (c *BlockStoreGRPCClient) Init(config map[string]string) error {
	_, err := c.grpcClient.Init(context.Background(), &proto.InitRequest{Plugin: c.plugin, Config: config})

	return err
}

// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
// and with the specified type and IOPS (if using provisioned IOPS).
func (c *BlockStoreGRPCClient) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	req := &proto.CreateVolumeRequest{
		Plugin:     c.plugin,
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
	res, err := c.grpcClient.GetVolumeInfo(context.Background(), &proto.GetVolumeInfoRequest{Plugin: c.plugin, VolumeID: volumeID, VolumeAZ: volumeAZ})
	if err != nil {
		return "", nil, err
	}

	var iops *int64
	if res.Iops != 0 {
		iops = &res.Iops
	}

	return res.VolumeType, iops, nil
}

// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
// set of tags to the snapshot.
func (c *BlockStoreGRPCClient) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	req := &proto.CreateSnapshotRequest{
		Plugin:   c.plugin,
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
	_, err := c.grpcClient.DeleteSnapshot(context.Background(), &proto.DeleteSnapshotRequest{Plugin: c.plugin, SnapshotID: snapshotID})

	return err
}

func (c *BlockStoreGRPCClient) GetVolumeID(pv runtime.Unstructured) (string, error) {
	encodedPV, err := json.Marshal(pv.UnstructuredContent())
	if err != nil {
		return "", err
	}

	req := &proto.GetVolumeIDRequest{
		Plugin:           c.plugin,
		PersistentVolume: encodedPV,
	}

	resp, err := c.grpcClient.GetVolumeID(context.Background(), req)
	if err != nil {
		return "", err
	}

	return resp.VolumeID, nil
}

func (c *BlockStoreGRPCClient) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	encodedPV, err := json.Marshal(pv.UnstructuredContent())
	if err != nil {
		return nil, err
	}

	req := &proto.SetVolumeIDRequest{
		Plugin:           c.plugin,
		PersistentVolume: encodedPV,
		VolumeID:         volumeID,
	}

	resp, err := c.grpcClient.SetVolumeID(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var updatedPV unstructured.Unstructured
	if err := json.Unmarshal(resp.PersistentVolume, &updatedPV); err != nil {
		return nil, err

	}

	return &updatedPV, nil
}

//////////////////////////////////////////////////////////////////////////////
// server code
//////////////////////////////////////////////////////////////////////////////

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

func (s *BlockStoreGRPCServer) getImpl(name string) (cloudprovider.BlockStore, error) {
	impl, err := s.mux.getHandler(name)
	if err != nil {
		return nil, err
	}

	blockStore, ok := impl.(cloudprovider.BlockStore)
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
