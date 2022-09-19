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

	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

// ItemSnapshotterGRPCServer implements the proto-generated ItemSnapshotterServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type ItemSnapshotterGRPCServer struct {
	mux *common.ServerMux
}

func (recv *ItemSnapshotterGRPCServer) getImpl(name string) (isv1.ItemSnapshotter, error) {
	impl, err := recv.mux.GetHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(isv1.ItemSnapshotter)
	if !ok {
		return nil, errors.Errorf("%T is not an item snapshotter", impl)
	}

	return itemAction, nil
}

func (recv *ItemSnapshotterGRPCServer) Init(c context.Context, req *proto.ItemSnapshotterInitRequest) (response *proto.Empty, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()

	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	err = impl.Init(req.Config)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	return &proto.Empty{}, nil
}

func (recv *ItemSnapshotterGRPCServer) AppliesTo(ctx context.Context, req *proto.ItemSnapshotterAppliesToRequest) (response *proto.ItemSnapshotterAppliesToResponse, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()

	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	resourceSelector, err := impl.AppliesTo()
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	return &proto.ItemSnapshotterAppliesToResponse{
		ResourceSelector: &proto.ResourceSelector{
			IncludedNamespaces: resourceSelector.IncludedNamespaces,
			ExcludedNamespaces: resourceSelector.ExcludedNamespaces,
			IncludedResources:  resourceSelector.IncludedResources,
			ExcludedResources:  resourceSelector.ExcludedResources,
			Selector:           resourceSelector.LabelSelector,
		},
	}, nil
}

func (recv *ItemSnapshotterGRPCServer) AlsoHandles(ctx context.Context, req *proto.AlsoHandlesRequest) (res *proto.AlsoHandlesResponse, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()

	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	var item unstructured.Unstructured
	var backup api.Backup

	if err := json.Unmarshal(req.Item, &item); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	if err := json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	ahi := isv1.AlsoHandlesInput{
		Item:   &item,
		Backup: &backup,
	}
	alsoHandles, err := impl.AlsoHandles(&ahi)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	res = &proto.AlsoHandlesResponse{}

	for _, item := range alsoHandles {
		res.HandledItems = append(res.HandledItems, resourceIdentifierToProto(item))
	}
	return res, nil
}

func (recv *ItemSnapshotterGRPCServer) SnapshotItem(ctx context.Context, req *proto.SnapshotItemRequest) (res *proto.SnapshotItemResponse, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()

	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	var item unstructured.Unstructured
	var backup api.Backup

	if err := json.Unmarshal(req.Item, &item); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	if err := json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	sii := isv1.SnapshotItemInput{
		Item:   &item,
		Params: req.Params,
		Backup: &backup,
	}
	sio, err := impl.SnapshotItem(ctx, &sii)

	// If the plugin implementation returned a nil updatedItem (meaning no modifications), reset updatedItem to the
	// original item.
	var updatedItemJSON []byte
	if sio.UpdatedItem == nil {
		updatedItemJSON = req.Item
	} else {
		updatedItemJSON, err = json.Marshal(sio.UpdatedItem.UnstructuredContent())
		if err != nil {
			return nil, common.NewGRPCError(errors.WithStack(err))
		}
	}
	res = &proto.SnapshotItemResponse{
		Item:             updatedItemJSON,
		SnapshotID:       sio.SnapshotID,
		SnapshotMetadata: sio.SnapshotMetadata,
	}
	res.AdditionalItems = packResourceIdentifiers(sio.AdditionalItems)
	res.HandledItems = packResourceIdentifiers(sio.HandledItems)
	return res, err
}

func (recv *ItemSnapshotterGRPCServer) Progress(ctx context.Context, req *proto.ProgressRequest) (res *proto.ProgressResponse, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()
	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	var backup api.Backup

	if err := json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	sipi := &isv1.ProgressInput{
		ItemID:     protoToResourceIdentifier(req.ItemID),
		SnapshotID: req.SnapshotID,
		Backup:     &backup,
	}

	sipo, err := impl.Progress(sipi)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	res = &proto.ProgressResponse{
		Phase:           string(sipo.Phase),
		ItemsCompleted:  sipo.ItemsCompleted,
		ItemsToComplete: sipo.ItemsToComplete,
		Started:         sipo.Started.Unix(),
		StartedNano:     sipo.Started.UnixNano(),
		Updated:         sipo.Updated.Unix(),
		UpdatedNano:     sipo.Updated.UnixNano(),
		Err:             sipo.Err,
	}
	return res, nil
}

func (recv *ItemSnapshotterGRPCServer) DeleteSnapshot(ctx context.Context, req *proto.DeleteItemSnapshotRequest) (empty *proto.Empty, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()
	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	var itemFromBackup unstructured.Unstructured
	if err := json.Unmarshal(req.ItemFromBackup, &itemFromBackup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}

	disi := isv1.DeleteSnapshotInput{
		SnapshotID:       req.SnapshotID,
		ItemFromBackup:   &itemFromBackup,
		SnapshotMetadata: req.Metadata,
		Params:           req.Params,
	}

	err = impl.DeleteSnapshot(ctx, &disi)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	return
}

func (recv *ItemSnapshotterGRPCServer) CreateItemFromSnapshot(ctx context.Context, req *proto.CreateItemFromSnapshotRequest) (res *proto.CreateItemFromSnapshotResponse, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()
	impl, err := recv.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	var snapshottedItem unstructured.Unstructured
	if err := json.Unmarshal(req.Item, &snapshottedItem); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}

	var itemFromBackup unstructured.Unstructured
	if err := json.Unmarshal(req.Item, &itemFromBackup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}

	var restore api.Restore

	if err := json.Unmarshal(req.Restore, &restore); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}

	cii := isv1.CreateItemInput{
		SnapshottedItem:  &snapshottedItem,
		SnapshotID:       req.SnapshotID,
		ItemFromBackup:   &itemFromBackup,
		SnapshotMetadata: req.SnapshotMetadata,
		Params:           req.Params,
		Restore:          &restore,
	}

	cio, err := impl.CreateItemFromSnapshot(ctx, &cii)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}

	var updatedItemJSON []byte
	if cio.UpdatedItem == nil {
		updatedItemJSON = req.Item
	} else {
		updatedItemJSON, err = json.Marshal(cio.UpdatedItem.UnstructuredContent())
		if err != nil {
			return nil, common.NewGRPCError(errors.WithStack(err))
		}
	}
	res = &proto.CreateItemFromSnapshotResponse{
		Item:        updatedItemJSON,
		SkipRestore: cio.SkipRestore,
	}
	res.AdditionalItems = packResourceIdentifiers(cio.AdditionalItems)

	return
}
