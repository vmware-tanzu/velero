/*
Copyright the Velero contributors.

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
	"context"
	"encoding/json"
	"time"

	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// NewItemSnapshotterPlugin constructs a ItemSnapshotterPlugin.
func NewItemSnapshotterPlugin(options ...PluginOption) *ItemSnapshotterPlugin {
	return &ItemSnapshotterPlugin{
		pluginBase: newPluginBase(options...),
	}
}

func newItemSnapshotterGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &ItemSnapshotterGRPCClient{
		clientBase: base,
		grpcClient: proto.NewItemSnapshotterClient(clientConn),
	}
}

// ItemSnapshotterGRPCClient implements the ItemSnapshotter interface and uses a
// gRPC client to make calls to the plugin server.
type ItemSnapshotterGRPCClient struct {
	*clientBase
	grpcClient proto.ItemSnapshotterClient
}

func (recv ItemSnapshotterGRPCClient) Init(config map[string]string) error {
	req := &proto.ItemSnapshotterInitRequest{
		Plugin: recv.plugin,
		Config: config,
	}

	_, err := recv.grpcClient.Init(context.Background(), req)
	return err
}

func (recv ItemSnapshotterGRPCClient) AppliesTo() (velero.ResourceSelector, error) {
	req := &proto.ItemSnapshotterAppliesToRequest{
		Plugin: recv.plugin,
	}

	res, err := recv.grpcClient.AppliesTo(context.Background(), req)
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

func (recv ItemSnapshotterGRPCClient) AlsoHandles(input *isv1.AlsoHandlesInput) ([]velero.ResourceIdentifier, error) {
	itemJSON, err := json.Marshal(input.Item.UnstructuredContent())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backupJSON, err := json.Marshal(input.Backup)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	req := &proto.AlsoHandlesRequest{
		Plugin: recv.plugin,
		Item:   itemJSON,
		Backup: backupJSON,
	}
	res, err := recv.grpcClient.AlsoHandles(context.Background(), req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	handledItems := unpackResourceIdentifiers(res.HandledItems)

	return handledItems, nil
}

func (recv ItemSnapshotterGRPCClient) SnapshotItem(ctx context.Context, input *isv1.SnapshotItemInput) (*isv1.SnapshotItemOutput, error) {
	itemJSON, err := json.Marshal(input.Item.UnstructuredContent())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backupJSON, err := json.Marshal(input.Backup)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	req := &proto.SnapshotItemRequest{
		Plugin: recv.plugin,
		Item:   itemJSON,
		Backup: backupJSON,
	}
	res, err := recv.grpcClient.SnapshotItem(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var updatedItem unstructured.Unstructured
	if err := json.Unmarshal(res.Item, &updatedItem); err != nil {
		return nil, errors.WithStack(err)
	}

	additionalItems := unpackResourceIdentifiers(res.AdditionalItems)
	handledItems := unpackResourceIdentifiers(res.HandledItems)

	sio := isv1.SnapshotItemOutput{
		UpdatedItem:      &updatedItem,
		SnapshotID:       res.SnapshotID,
		SnapshotMetadata: res.SnapshotMetadata,
		AdditionalItems:  additionalItems,
		HandledItems:     handledItems,
	}
	return &sio, nil
}

func (recv ItemSnapshotterGRPCClient) Progress(input *isv1.ProgressInput) (*isv1.ProgressOutput, error) {
	backupJSON, err := json.Marshal(input.Backup)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	req := &proto.ProgressRequest{
		Plugin:     recv.plugin,
		ItemID:     resourceIdentifierToProto(input.ItemID),
		SnapshotID: input.SnapshotID,
		Backup:     backupJSON,
	}

	res, err := recv.grpcClient.Progress(context.Background(), req)

	if err != nil {
		return nil, errors.WithStack(err)
	}
	// Validate phase

	phase, err := isv1.SnapshotPhaseFromString(res.Phase)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	up := isv1.ProgressOutput{
		Phase:           phase,
		Err:             res.Err,
		ItemsCompleted:  res.ItemsCompleted,
		ItemsToComplete: res.ItemsToComplete,
		Started:         time.Unix(res.Started, res.StartedNano),
		Updated:         time.Unix(res.Updated, res.UpdatedNano),
	}
	return &up, nil
}

func (recv ItemSnapshotterGRPCClient) DeleteSnapshot(ctx context.Context, input *isv1.DeleteSnapshotInput) error {
	req := &proto.DeleteItemSnapshotRequest{
		Plugin:     recv.plugin,
		Params:     input.Params,
		SnapshotID: input.SnapshotID,
	}
	_, err := recv.grpcClient.DeleteSnapshot(ctx, req) // Returns Empty as first arg so just ignore

	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (recv ItemSnapshotterGRPCClient) CreateItemFromSnapshot(ctx context.Context, input *isv1.CreateItemInput) (*isv1.CreateItemOutput, error) {
	itemJSON, err := json.Marshal(input.SnapshottedItem.UnstructuredContent())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	itemFromBackupJSON, err := json.Marshal(input.ItemFromBackup.UnstructuredContent())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	restoreJSON, err := json.Marshal(input.Restore)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	req := &proto.CreateItemFromSnapshotRequest{
		Plugin:           recv.plugin,
		Item:             itemJSON,
		SnapshotID:       input.SnapshotID,
		ItemFromBackup:   itemFromBackupJSON,
		SnapshotMetadata: input.SnapshotMetadata,
		Params:           input.Params,
		Restore:          restoreJSON,
	}

	res, err := recv.grpcClient.CreateItemFromSnapshot(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var updatedItem unstructured.Unstructured
	if err := json.Unmarshal(res.Item, &updatedItem); err != nil {
		return nil, errors.WithStack(err)
	}

	additionalItems := unpackResourceIdentifiers(res.AdditionalItems)

	cio := isv1.CreateItemOutput{
		UpdatedItem:     &updatedItem,
		AdditionalItems: additionalItems,
		SkipRestore:     res.SkipRestore,
	}
	return &cio, nil
}
