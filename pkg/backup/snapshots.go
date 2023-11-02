package backup

import (
	"context"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotv1listers "github.com/kubernetes-csi/external-snapshotter/client/v4/listers/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

// Common function to update the status of CSI snapshots
// returns VolumeSnapshot, VolumeSnapshotContent, VolumeSnapshotClasses referenced
func UpdateBackupCSISnapshotsStatus(client kbclient.Client, volumeSnapshotLister snapshotv1listers.VolumeSnapshotLister, backup *velerov1api.Backup, backupLog logrus.FieldLogger) (volumeSnapshots []snapshotv1api.VolumeSnapshot, volumeSnapshotContents []snapshotv1api.VolumeSnapshotContent, volumeSnapshotClasses []snapshotv1api.VolumeSnapshotClass) {
	if boolptr.IsSetToTrue(backup.Spec.SnapshotMoveData) {
		backupLog.Info("backup SnapshotMoveData is set to true, skip VolumeSnapshot resource persistence.")
	} else if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		selector := label.NewSelectorForBackup(backup.Name)
		vscList := &snapshotv1api.VolumeSnapshotContentList{}

		if volumeSnapshotLister != nil {
			tmpVSs, err := volumeSnapshotLister.List(label.NewSelectorForBackup(backup.Name))
			if err != nil {
				backupLog.Error(err)
			}
			for _, vs := range tmpVSs {
				volumeSnapshots = append(volumeSnapshots, *vs)
			}
		}

		err := client.List(context.Background(), vscList, &kbclient.ListOptions{LabelSelector: selector})
		if err != nil {
			backupLog.Error(err)
		}
		if len(vscList.Items) >= 0 {
			volumeSnapshotContents = vscList.Items
		}

		vsClassSet := sets.NewString()
		for index := range volumeSnapshotContents {
			// persist the volumesnapshotclasses referenced by vsc
			if volumeSnapshotContents[index].Spec.VolumeSnapshotClassName != nil && !vsClassSet.Has(*volumeSnapshotContents[index].Spec.VolumeSnapshotClassName) {
				vsClass := &snapshotv1api.VolumeSnapshotClass{}
				if err := client.Get(context.TODO(), kbclient.ObjectKey{Name: *volumeSnapshotContents[index].Spec.VolumeSnapshotClassName}, vsClass); err != nil {
					backupLog.Error(err)
				} else {
					vsClassSet.Insert(*volumeSnapshotContents[index].Spec.VolumeSnapshotClassName)
					volumeSnapshotClasses = append(volumeSnapshotClasses, *vsClass)
				}
			}
		}
		backup.Status.CSIVolumeSnapshotsAttempted = len(volumeSnapshots)
		csiVolumeSnapshotsCompleted := 0
		for _, vs := range volumeSnapshots {
			if vs.Status != nil && boolptr.IsSetToTrue(vs.Status.ReadyToUse) {
				csiVolumeSnapshotsCompleted++
			}
		}
		backup.Status.CSIVolumeSnapshotsCompleted = csiVolumeSnapshotsCompleted
	}
	return volumeSnapshots, volumeSnapshotContents, volumeSnapshotClasses
}
