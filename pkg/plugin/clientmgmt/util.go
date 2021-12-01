package clientmgmt

import (
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

// ItemSnapshotterForSnapshotFromResolved - Gets the ItemSnapshotter that was used to take a snapshot.  We have two for different array types for convenience since
// Go doesn't allow us to cast arrays easily
func ItemSnapshotterForSnapshotFromResolved(itemSnapshot *volume.ItemSnapshot, resolvedItemSnapshotters []framework.ItemSnapshotterResolvedAction) isv1.ItemSnapshotter {
	var itemSnapshotters []isv1.ItemSnapshotter
	for _, resolvedAction := range resolvedItemSnapshotters {
		itemSnapshotter, ok := resolvedAction.ItemSnapshotter.(LocalItemSnapshotter)
		if ok {
			itemSnapshotters = append(itemSnapshotters, itemSnapshotter)
		}
	}
	return ItemSnapshotterForSnapshot(itemSnapshot, itemSnapshotters)
}

func ItemSnapshotterForSnapshot(itemSnapshot *volume.ItemSnapshot, itemSnapshotters []isv1.ItemSnapshotter) isv1.ItemSnapshotter {
	for _, checkItemSnapshotter := range itemSnapshotters {
		localCheckItemSnapshotter, ok := checkItemSnapshotter.(LocalItemSnapshotter)
		if ok {
			if itemSnapshot.Spec.ItemSnapshotter == localCheckItemSnapshotter.GetName() {
				return checkItemSnapshotter
			}
		}
	}
	return nil
}
