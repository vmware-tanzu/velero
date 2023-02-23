/*
Copyright The Velero Contributors.

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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/podvolume"
	"github.com/vmware-tanzu/velero/pkg/repository"
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/provider"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// For unit test to mock function
var NewUploaderProviderFunc = provider.NewUploaderProvider

// PodVolumeBackupReconciler reconciles a PodVolumeBackup object
type PodVolumeBackupReconciler struct {
	Scheme           *runtime.Scheme
	Client           client.Client
	Clock            clocks.WithTickerAndDelayedExecution
	Metrics          *metrics.ServerMetrics
	CredentialGetter *credentials.CredentialGetter
	NodeName         string
	FileSystem       filesystem.Interface
	Log              logrus.FieldLogger
}

type BackupProgressUpdater struct {
	PodVolumeBackup *velerov1api.PodVolumeBackup
	Log             logrus.FieldLogger
	Ctx             context.Context
	Cli             client.Client
}

// +kubebuilder:rbac:groups=velero.io,resources=podvolumebackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=podvolumebackups/status,verbs=get;update;patch
func (r *PodVolumeBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithFields(logrus.Fields{
		"controller":      "podvolumebackup",
		"podvolumebackup": req.NamespacedName,
	})

	var pvb velerov1api.PodVolumeBackup
	if err := r.Client.Get(ctx, req.NamespacedName, &pvb); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find PodVolumeBackup")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "getting PodVolumeBackup")
	}
	if len(pvb.OwnerReferences) == 1 {
		log = log.WithField(
			"backup",
			fmt.Sprintf("%s/%s", req.Namespace, pvb.OwnerReferences[0].Name),
		)
	}

	log.Info("PodVolumeBackup starting")

	// Only process items for this node.
	if pvb.Spec.Node != r.NodeName {
		return ctrl.Result{}, nil
	}

	switch pvb.Status.Phase {
	case "", velerov1api.PodVolumeBackupPhaseNew:
		// Only process new items.
	default:
		log.Debug("PodVolumeBackup is not new, not processing")
		return ctrl.Result{}, nil
	}

	r.Metrics.RegisterPodVolumeBackupEnqueue(r.NodeName)

	// Update status to InProgress.
	original := pvb.DeepCopy()
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseInProgress
	pvb.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
	if err := r.Client.Patch(ctx, &pvb, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("error updating PodVolumeBackup status")
		return ctrl.Result{}, err
	}

	var pod corev1.Pod
	podNamespacedName := client.ObjectKey{
		Namespace: pvb.Spec.Pod.Namespace,
		Name:      pvb.Spec.Pod.Name,
	}
	if err := r.Client.Get(ctx, podNamespacedName, &pod); err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, fmt.Sprintf("getting pod %s/%s", pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name), log)
	}

	volDir, err := kube.GetVolumeDirectory(ctx, log, &pod, pvb.Spec.Volume, r.Client)
	if err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, "getting volume directory name", log)
	}

	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(pvb.Spec.Pod.UID), volDir)
	log.WithField("pathGlob", pathGlob).Debug("Looking for path matching glob")

	path, err := kube.SinglePathMatch(pathGlob, r.FileSystem, log)
	if err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, "identifying unique volume path on host", log)
	}
	log.WithField("path", path).Debugf("Found path matching glob")

	backupLocation := &velerov1api.BackupStorageLocation{}
	if err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: pvb.Namespace,
		Name:      pvb.Spec.BackupStorageLocation,
	}, backupLocation); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error getting backup storage location")
	}

	backupRepo, err := repository.GetBackupRepository(ctx, r.Client, pvb.Namespace, repository.BackupRepositoryKey{
		VolumeNamespace: pvb.Spec.Pod.Namespace,
		BackupLocation:  pvb.Spec.BackupStorageLocation,
		RepositoryType:  podvolume.GetPvbRepositoryType(&pvb),
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error getting backup repository")
	}

	var uploaderProv provider.Provider
	uploaderProv, err = NewUploaderProviderFunc(ctx, r.Client, pvb.Spec.UploaderType, pvb.Spec.RepoIdentifier,
		backupLocation, backupRepo, r.CredentialGetter, repokey.RepoKeySelector(), log)
	if err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, "error creating uploader", log)
	}

	// If this is a PVC, look for the most recent completed pod volume backup for it and get
	// its snapshot ID to do new backup based on it. Without this,
	// if the pod using the PVC (and therefore the directory path under /host_pods/) has
	// changed since the PVC's last backup, for backup, it will not be able to identify a suitable
	// parent snapshot to use, and will have to do a full rescan of the contents of the PVC.
	var parentSnapshotID string
	if pvcUID, ok := pvb.Labels[velerov1api.PVCUIDLabel]; ok {
		parentSnapshotID = r.getParentSnapshot(ctx, log, pvcUID, &pvb)
		if parentSnapshotID == "" {
			log.Info("No parent snapshot found for PVC, not based on parent snapshot for this backup")
		} else {
			log.WithField("parentSnapshotID", parentSnapshotID).Info("Based on parent snapshot for this backup")
		}
	}

	defer func() {
		if err := uploaderProv.Close(ctx); err != nil {
			log.Errorf("failed to close uploader provider with error %v", err)
		}
	}()

	snapshotID, emptySnapshot, err := uploaderProv.RunBackup(ctx, path, pvb.Spec.Tags, parentSnapshotID, r.NewBackupProgressUpdater(&pvb, log, ctx))
	if err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, fmt.Sprintf("running backup, stderr=%v", err), log)
	}

	// Update status to Completed with path & snapshot ID.
	original = pvb.DeepCopy()
	pvb.Status.Path = path
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseCompleted
	pvb.Status.SnapshotID = snapshotID
	pvb.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
	if emptySnapshot {
		pvb.Status.Message = "volume was empty so no snapshot was taken"
	}
	if err = r.Client.Patch(ctx, &pvb, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("error updating PodVolumeBackup status")
		return ctrl.Result{}, err
	}

	latencyDuration := pvb.Status.CompletionTimestamp.Time.Sub(pvb.Status.StartTimestamp.Time)
	latencySeconds := float64(latencyDuration / time.Second)
	backupName := fmt.Sprintf("%s/%s", req.Namespace, pvb.OwnerReferences[0].Name)
	generateOpName := fmt.Sprintf("%s-%s-%s-%s-%s-backup", pvb.Name, backupRepo.Name, pvb.Spec.BackupStorageLocation, pvb.Namespace, pvb.Spec.UploaderType)
	r.Metrics.ObservePodVolumeOpLatency(r.NodeName, req.Name, generateOpName, backupName, latencySeconds)
	r.Metrics.RegisterPodVolumeOpLatencyGauge(r.NodeName, req.Name, generateOpName, backupName, latencySeconds)
	r.Metrics.RegisterPodVolumeBackupDequeue(r.NodeName)

	log.Info("PodVolumeBackup completed")
	return ctrl.Result{}, nil
}

// SetupWithManager registers the PVB controller.
func (r *PodVolumeBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.PodVolumeBackup{}).
		Complete(r)
}

// getParentSnapshot finds the most recent completed PodVolumeBackup for the
// specified PVC and returns its snapshot ID. Any errors encountered are
// logged but not returned since they do not prevent a backup from proceeding.
func (r *PodVolumeBackupReconciler) getParentSnapshot(ctx context.Context, log logrus.FieldLogger, pvcUID string, podVolumeBackup *velerov1api.PodVolumeBackup) string {
	log = log.WithField("pvcUID", pvcUID)
	log.Infof("Looking for most recent completed PodVolumeBackup for this PVC")

	listOpts := &client.ListOptions{
		Namespace: podVolumeBackup.Namespace,
	}
	matchingLabels := client.MatchingLabels(map[string]string{velerov1api.PVCUIDLabel: pvcUID})
	matchingLabels.ApplyToList(listOpts)

	var pvbList velerov1api.PodVolumeBackupList
	if err := r.Client.List(ctx, &pvbList, listOpts); err != nil {
		log.WithError(errors.WithStack(err)).Error("getting list of podvolumebackups for this PVC")
	}

	// Go through all the podvolumebackups for the PVC and look for the most
	// recent completed one to use as the parent.
	var mostRecentPVB velerov1api.PodVolumeBackup
	for _, pvb := range pvbList.Items {
		if pvb.Spec.UploaderType != podVolumeBackup.Spec.UploaderType {
			continue
		}
		if pvb.Status.Phase != velerov1api.PodVolumeBackupPhaseCompleted {
			continue
		}

		if podVolumeBackup.Spec.BackupStorageLocation != pvb.Spec.BackupStorageLocation {
			// Check the backup storage location is the same as spec in order to
			// support backup to multiple backup-locations. Otherwise, there exists
			// a case that backup volume snapshot to the second location would
			// failed, since the founded parent ID is only valid for the first
			// backup location, not the second backup location. Also, the second
			// backup should not use the first backup parent ID since its for the
			// first backup location only.
			continue
		}

		if mostRecentPVB.Status == (velerov1api.PodVolumeBackupStatus{}) || pvb.Status.StartTimestamp.After(mostRecentPVB.Status.StartTimestamp.Time) {
			mostRecentPVB = pvb
		}
	}

	if mostRecentPVB.Status == (velerov1api.PodVolumeBackupStatus{}) {
		log.Info("No completed PodVolumeBackup found for PVC")
		return ""
	}

	log.WithFields(map[string]interface{}{
		"parentPodVolumeBackup": mostRecentPVB.Name,
		"parentSnapshotID":      mostRecentPVB.Status.SnapshotID,
	}).Info("Found most recent completed PodVolumeBackup for PVC")

	return mostRecentPVB.Status.SnapshotID
}

func (r *PodVolumeBackupReconciler) updateStatusToFailed(ctx context.Context, pvb *velerov1api.PodVolumeBackup, err error, msg string, log logrus.FieldLogger) (ctrl.Result, error) {
	if err = UpdatePVBStatusToFailed(r.Client, ctx, pvb, errors.WithMessage(err, msg).Error(), r.Clock.Now()); err != nil {
		log.WithError(err).Error("error updating PodVolumeBackup status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func UpdatePVBStatusToFailed(c client.Client, ctx context.Context, pvb *velerov1api.PodVolumeBackup, errString string, time time.Time) error {
	original := pvb.DeepCopy()
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseFailed
	pvb.Status.Message = errString
	pvb.Status.CompletionTimestamp = &metav1.Time{Time: time}

	return c.Patch(ctx, pvb, client.MergeFrom(original))
}

func (r *PodVolumeBackupReconciler) NewBackupProgressUpdater(pvb *velerov1api.PodVolumeBackup, log logrus.FieldLogger, ctx context.Context) *BackupProgressUpdater {
	return &BackupProgressUpdater{pvb, log, ctx, r.Client}
}

// UpdateProgress which implement ProgressUpdater interface to update pvb progress status
func (b *BackupProgressUpdater) UpdateProgress(p *uploader.UploaderProgress) {
	original := b.PodVolumeBackup.DeepCopy()
	b.PodVolumeBackup.Status.Progress = velerov1api.PodVolumeOperationProgress{TotalBytes: p.TotalBytes, BytesDone: p.BytesDone}
	if b.Cli == nil {
		b.Log.Errorf("failed to update backup pod %s volume %s progress with uninitailize client", b.PodVolumeBackup.Spec.Pod.Name, b.PodVolumeBackup.Spec.Volume)
		return
	}
	if err := b.Cli.Patch(b.Ctx, b.PodVolumeBackup, client.MergeFrom(original)); err != nil {
		b.Log.Errorf("update backup pod %s volume %s progress with %v", b.PodVolumeBackup.Spec.Pod.Name, b.PodVolumeBackup.Spec.Volume, err)
	}
}
