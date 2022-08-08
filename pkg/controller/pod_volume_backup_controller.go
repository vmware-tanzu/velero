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
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// BackupExecuter runs backups.
type BackupExecuter interface {
	RunBackup(*restic.Command, logrus.FieldLogger, func(velerov1api.PodVolumeOperationProgress)) (string, string, error)
	GetSnapshotID(*restic.Command) (string, error)
}

// PodVolumeBackupReconciler reconciles a PodVolumeBackup object
type PodVolumeBackupReconciler struct {
	Scheme         *runtime.Scheme
	Client         client.Client
	Clock          clock.Clock
	Metrics        *metrics.ServerMetrics
	CredsFileStore credentials.FileStore
	NodeName       string
	FileSystem     filesystem.Interface
	ResticExec     BackupExecuter
	Log            logrus.FieldLogger
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

	var resticDetails resticDetails
	resticCmd, err := r.buildResticCommand(ctx, log, &pvb, &pod, &resticDetails)
	if err != nil {
		return r.updateStatusToFailed(ctx, &pvb, err, "building Restic command", log)
	}

	defer func() {
		os.Remove(resticDetails.credsFile)
		os.Remove(resticDetails.caCertFile)
	}()

	backupLocation := &velerov1api.BackupStorageLocation{}
	if err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: pvb.Namespace,
		Name:      pvb.Spec.BackupStorageLocation,
	}, backupLocation); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error getting backup storage location")
	}

	// #4820: restrieve insecureSkipTLSVerify from BSL configuration for
	// AWS plugin. If nothing is return, that means insecureSkipTLSVerify
	// is not enable for Restic command.
	skipTLSRet := restic.GetInsecureSkipTLSVerifyFromBSL(backupLocation, log)
	if len(skipTLSRet) > 0 {
		resticCmd.ExtraFlags = append(resticCmd.ExtraFlags, skipTLSRet)
	}

	var stdout, stderr string

	var emptySnapshot bool
	stdout, stderr, err = r.ResticExec.RunBackup(resticCmd, log, r.updateBackupProgressFunc(&pvb, log))
	if err != nil {
		if strings.Contains(stderr, "snapshot is empty") {
			emptySnapshot = true
		} else {
			return r.updateStatusToFailed(ctx, &pvb, err, fmt.Sprintf("running Restic backup, stderr=%s", stderr), log)
		}
	}
	log.Debugf("Ran command=%s, stdout=%s, stderr=%s", resticCmd.String(), stdout, stderr)

	var snapshotID string
	if !emptySnapshot {
		cmd := restic.GetSnapshotCommand(pvb.Spec.RepoIdentifier, resticDetails.credsFile, pvb.Spec.Tags)
		cmd.Env = resticDetails.envs
		cmd.CACertFile = resticDetails.caCertFile

		// #4820: also apply the insecureTLS flag to Restic snapshots command
		if len(skipTLSRet) > 0 {
			cmd.ExtraFlags = append(cmd.ExtraFlags, skipTLSRet)
		}

		snapshotID, err = r.ResticExec.GetSnapshotID(cmd)
		if err != nil {
			return r.updateStatusToFailed(ctx, &pvb, err, "getting snapshot id", log)
		}
	}

	// Update status to Completed with path & snapshot ID.
	original = pvb.DeepCopy()
	pvb.Status.Path = resticDetails.path
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
	r.Metrics.ObserveResticOpLatency(r.NodeName, req.Name, resticCmd.Command, backupName, latencySeconds)
	r.Metrics.RegisterResticOpLatencyGauge(r.NodeName, req.Name, resticCmd.Command, backupName, latencySeconds)
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
// specified PVC and returns its Restic snapshot ID. Any errors encountered are
// logged but not returned since they do not prevent a backup from proceeding.
func (r *PodVolumeBackupReconciler) getParentSnapshot(ctx context.Context, log logrus.FieldLogger, pvbNamespace, pvcUID, bsl string) string {
	log = log.WithField("pvcUID", pvcUID)
	log.Infof("Looking for most recent completed PodVolumeBackup for this PVC")

	listOpts := &client.ListOptions{
		Namespace: pvbNamespace,
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
		if pvb.Status.Phase != velerov1api.PodVolumeBackupPhaseCompleted {
			continue
		}

		if bsl != pvb.Spec.BackupStorageLocation {
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
	original := pvb.DeepCopy()
	pvb.Status.Phase = velerov1api.PodVolumeBackupPhaseFailed
	pvb.Status.Message = errors.WithMessage(err, msg).Error()
	pvb.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}

	if err = r.Client.Patch(ctx, pvb, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("error updating PodVolumeBackup status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type resticDetails struct {
	credsFile, caCertFile string
	envs                  []string
	path                  string
}

func (r *PodVolumeBackupReconciler) buildResticCommand(ctx context.Context, log *logrus.Entry, pvb *velerov1api.PodVolumeBackup, pod *corev1.Pod, details *resticDetails) (*restic.Command, error) {
	volDir, err := kube.GetVolumeDirectory(ctx, log, pod, pvb.Spec.Volume, r.Client)
	if err != nil {
		return nil, errors.Wrap(err, "getting volume directory name")
	}

	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(pvb.Spec.Pod.UID), volDir)
	log.WithField("pathGlob", pathGlob).Debug("Looking for path matching glob")

	path, err := kube.SinglePathMatch(pathGlob, r.FileSystem, log)
	if err != nil {
		return nil, errors.Wrap(err, "identifying unique volume path on host")
	}
	log.WithField("path", path).Debugf("Found path matching glob")

	// Temporary credentials.
	details.credsFile, err = r.CredsFileStore.Path(repokey.RepoKeySelector())
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary Restic credentials file")
	}

	cmd := restic.BackupCommand(pvb.Spec.RepoIdentifier, details.credsFile, path, pvb.Spec.Tags)

	backupLocation := &velerov1api.BackupStorageLocation{}
	if err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: pvb.Namespace,
		Name:      pvb.Spec.BackupStorageLocation,
	}, backupLocation); err != nil {
		return nil, errors.Wrap(err, "getting backup storage location")
	}

	// If there's a caCert on the ObjectStorage, write it to disk so that it can
	// be passed to Restic.
	if backupLocation.Spec.ObjectStorage != nil &&
		backupLocation.Spec.ObjectStorage.CACert != nil {

		details.caCertFile, err = restic.TempCACertFile(backupLocation.Spec.ObjectStorage.CACert, pvb.Spec.BackupStorageLocation, r.FileSystem)
		if err != nil {
			log.WithError(err).Error("creating temporary caCert file")
		}
	}
	cmd.CACertFile = details.caCertFile

	details.envs, err = restic.CmdEnv(backupLocation, r.CredsFileStore)
	if err != nil {
		return nil, errors.Wrap(err, "setting Restic command environment")
	}
	cmd.Env = details.envs

	// If this is a PVC, look for the most recent completed PodVolumeBackup for
	// it and get its Restic snapshot ID to use as the value of the `--parent`
	// flag. Without this, if the pod using the PVC (and therefore the directory
	// path under /host_pods/) has changed since the PVC's last backup, Restic
	// will not be able to identify a suitable parent snapshot to use, and will
	// have to do a full rescan of the contents of the PVC.
	if pvcUID, ok := pvb.Labels[velerov1api.PVCUIDLabel]; ok {
		parentSnapshotID := r.getParentSnapshot(ctx, log, pvb.Namespace, pvcUID, pvb.Spec.BackupStorageLocation)
		if parentSnapshotID == "" {
			log.Info("No parent snapshot found for PVC, not using --parent flag for this backup")
		} else {
			log.WithField("parentSnapshotID", parentSnapshotID).
				Info("Setting --parent flag for this backup")
			cmd.ExtraFlags = append(cmd.ExtraFlags, fmt.Sprintf("--parent=%s", parentSnapshotID))
		}
	}

	return cmd, nil
}
