/*
Copyright 2017, 2019 the Velero contributors.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

// NewVolumeSnapshotterPlugin constructs a VolumeSnapshotterPlugin.
func NewVolumeSnapshotterPlugin(options ...PluginOption) *VolumeSnapshotterPlugin {
	return &VolumeSnapshotterPlugin{
		pluginBase: newPluginBase(options...),
	}
}

// VolumeSnapshotterGRPCClient implements the cloudprovider.VolumeSnapshotter interface and uses a
// gRPC client to make calls to the plugin server.
type VolumeSnapshotterGRPCClient struct {
	*clientBase
	grpcClient proto.VolumeSnapshotterClient
}

func newVolumeSnapshotterGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &VolumeSnapshotterGRPCClient{
		clientBase: base,
		grpcClient: proto.NewVolumeSnapshotterClient(clientConn),
	}
}

// Init prepares the VolumeSnapshotter for usage using the provided map of
// configuration key-value pairs. It returns an error if the VolumeSnapshotter
// cannot be initialized from the provided config.
func (c *VolumeSnapshotterGRPCClient) Init(config map[string]string) error {
	req := &proto.VolumeSnapshotterInitRequest{
		Plugin: c.plugin,
		Config: config,
	}

	if _, err := c.grpcClient.Init(context.Background(), req); err != nil {
		return fromGRPCError(err)
	}

	return nil
}

// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
// and with the specified type and IOPS (if using provisioned IOPS).
func (c *VolumeSnapshotterGRPCClient) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
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
		return "", fromGRPCError(err)
	}

	return res.VolumeID, nil
}

// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for a specified block
// volume.
func (c *VolumeSnapshotterGRPCClient) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	req := &proto.GetVolumeInfoRequest{
		Plugin:   c.plugin,
		VolumeID: volumeID,
		VolumeAZ: volumeAZ,
	}

	res, err := c.grpcClient.GetVolumeInfo(context.Background(), req)
	if err != nil {
		return "", nil, fromGRPCError(err)
	}

	var iops *int64
	if res.Iops != 0 {
		iops = &res.Iops
	}

	return res.VolumeType, iops, nil
}

// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
// set of tags to the snapshot.
func (c *VolumeSnapshotterGRPCClient) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	req := &proto.CreateSnapshotRequest{
		Plugin:   c.plugin,
		VolumeID: volumeID,
		VolumeAZ: volumeAZ,
		Tags:     tags,
	}

	res, err := c.grpcClient.CreateSnapshot(context.Background(), req)
	if err != nil {
		return "", fromGRPCError(err)
	}

	return res.SnapshotID, nil
}

// DeleteSnapshot deletes the specified volume snapshot.
func (c *VolumeSnapshotterGRPCClient) DeleteSnapshot(snapshotID string) error {
	req := &proto.DeleteSnapshotRequest{
		Plugin:     c.plugin,
		SnapshotID: snapshotID,
	}

	if _, err := c.grpcClient.DeleteSnapshot(context.Background(), req); err != nil {
		return fromGRPCError(err)
	}

	return nil
}

func (c *VolumeSnapshotterGRPCClient) GetVolumeID(pv runtime.Unstructured) (string, error) {
	encodedPV, err := json.Marshal(pv.UnstructuredContent())
	if err != nil {
		return "", errors.WithStack(err)
	}

	req := &proto.GetVolumeIDRequest{
		Plugin:           c.plugin,
		PersistentVolume: encodedPV,
	}

	resp, err := c.grpcClient.GetVolumeID(context.Background(), req)
	if err != nil {
		return "", fromGRPCError(err)
	}

	return resp.VolumeID, nil
}

func (c *VolumeSnapshotterGRPCClient) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	encodedPV, err := json.Marshal(pv.UnstructuredContent())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	req := &proto.SetVolumeIDRequest{
		Plugin:           c.plugin,
		PersistentVolume: encodedPV,
		VolumeID:         volumeID,
	}

	resp, err := c.grpcClient.SetVolumeID(context.Background(), req)
	if err != nil {
		return nil, fromGRPCError(err)
	}

	var updatedPV unstructured.Unstructured
	if err := json.Unmarshal(resp.PersistentVolume, &updatedPV); err != nil {
		return nil, errors.WithStack(err)
	}

	return &updatedPV, nil
}
