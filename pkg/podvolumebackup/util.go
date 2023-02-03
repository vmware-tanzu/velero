package podvolumebackup

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func UpdateStatusToFailed(client pkgclient.Client, ctx context.Context, pvb *velerov1api.PodVolumeBackup, errString string, time time.Time) error {
	original := pvb.DeepCopy()
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseFailed
	pvb.Status.Message = errString
	pvb.Status.CompletionTimestamp = &metav1.Time{Time: time}

	return client.Patch(ctx, pvb, pkgclient.MergeFrom(original))
}
