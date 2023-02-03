package podvolumerestore

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

func UpdateStatusToFailed(client pkgclient.Client, ctx context.Context, pvr *velerov1api.PodVolumeRestore, errString string, time time.Time) error {
	original := pvr.DeepCopy()
	pvr.Status.Phase = velerov1api.PodVolumeRestorePhaseFailed
	pvr.Status.Message = errString
	pvr.Status.CompletionTimestamp = &metav1.Time{Time: time}

	return client.Patch(ctx, pvr, pkgclient.MergeFrom(original))
}
