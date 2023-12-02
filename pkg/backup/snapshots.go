package backup

import (
	"context"
	"fmt"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

// Tracks volume snapshots for this backup.
// Completion status will be updated in backup_operations_controller.go via ProgressOperations information.
var (
	backupVolumeSnapshotTracker map[string]map[string]bool // backup name -> namespace/volume snapshot name -> completion status
)

func TrackVolumeSnapshotsForBackup(backup *velerov1api.Backup, volumeSnapshots []snapshotv1api.VolumeSnapshot, backupLog logrus.FieldLogger) {
	if backupVolumeSnapshotTracker == nil {
		backupVolumeSnapshotTracker = make(map[string]map[string]bool, 0)
	}
	if backupVolumeSnapshotTracker[backup.Name] == nil {
		backupVolumeSnapshotTracker[backup.Name] = make(map[string]bool, 0)
	}
	for _, vs := range volumeSnapshots {
		backupVolumeSnapshotTracker[backup.Name][VolumeSnapshotStatusTrackerName(&vs)] = false
	}
	backupLog.Infof("Tracking %d volume snapshots for backup %s", len(volumeSnapshots), backup.Name)
}

func VolumeSnapshotStatusTrackerName(vs *snapshotv1api.VolumeSnapshot) string {
	return fmt.Sprintf("%s/%s", vs.Namespace, vs.Name)
}

func GetVolumeSnapshotTrackerNamesForBackupName(backupName string) map[string]bool {
	return backupVolumeSnapshotTracker[backupName]
}

func UntrackVolumeSnapshotsForBackup(backup *velerov1api.Backup) {
	if backupVolumeSnapshotTracker == nil {
		return
	}
	delete(backupVolumeSnapshotTracker, backup.Name)
}

// Common function to update the status of CSI snapshots
// returns VolumeSnapshot, VolumeSnapshotContent, VolumeSnapshotClasses referenced
func UpdateBackupCSISnapshotsStatus(client kbclient.Client, globalCRClient kbclient.Client, backup *velerov1api.Backup, backupLog logrus.FieldLogger) (volumeSnapshots []snapshotv1api.VolumeSnapshot, volumeSnapshotContents []snapshotv1api.VolumeSnapshotContent, volumeSnapshotClasses []snapshotv1api.VolumeSnapshotClass) {
	if boolptr.IsSetToTrue(backup.Spec.SnapshotMoveData) {
		backupLog.Info("backup SnapshotMoveData is set to true, skip VolumeSnapshot resource persistence.")
	} else if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		selector := label.NewSelectorForBackup(backup.Name)
		vscList := &snapshotv1api.VolumeSnapshotContentList{}

		vsList := new(snapshotv1api.VolumeSnapshotList)
		err := globalCRClient.List(context.TODO(), vsList, &kbclient.ListOptions{
			LabelSelector: label.NewSelectorForBackup(backup.Name),
		})
		if err != nil {
			backupLog.Error(err)
		}

		volumeSnapshots = append(volumeSnapshots, vsList.Items...)

		if err := client.List(context.Background(), vscList, &kbclient.ListOptions{LabelSelector: selector}); err != nil {
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
