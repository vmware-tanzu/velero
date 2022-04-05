package clientmgmt

import (
	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

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
