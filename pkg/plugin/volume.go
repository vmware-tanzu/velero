package plugin

import (
	"encoding/json"

	"github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

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
