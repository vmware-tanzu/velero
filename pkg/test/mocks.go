package test

import (
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	snapshotv1listers "github.com/kubernetes-csi/external-snapshotter/client/v7/listers/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// VolumeSnapshotLister helps list VolumeSnapshots.
// All objects returned here must be treated as read-only.
//
//go:generate mockery --name VolumeSnapshotLister
type VolumeSnapshotLister interface {
	// List lists all VolumeSnapshots in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*snapshotv1.VolumeSnapshot, err error)
	// VolumeSnapshots returns an object that can list and get VolumeSnapshots.
	VolumeSnapshots(namespace string) snapshotv1listers.VolumeSnapshotNamespaceLister
	snapshotv1listers.VolumeSnapshotListerExpansion
}
