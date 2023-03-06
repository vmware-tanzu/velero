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
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/podvolume"
	"github.com/vmware-tanzu/velero/pkg/repository"
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/restorehelper"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/provider"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

func NewPodVolumeRestoreReconciler(logger logrus.FieldLogger, client client.Client, credentialGetter *credentials.CredentialGetter) *PodVolumeRestoreReconciler {
	return &PodVolumeRestoreReconciler{
		Client:           client,
		logger:           logger.WithField("controller", "PodVolumeRestore"),
		credentialGetter: credentialGetter,
		fileSystem:       filesystem.NewFileSystem(),
		clock:            &clocks.RealClock{},
	}
}

type PodVolumeRestoreReconciler struct {
	client.Client
	logger           logrus.FieldLogger
	credentialGetter *credentials.CredentialGetter
	fileSystem       filesystem.Interface
	clock            clocks.WithTickerAndDelayedExecution
}

type RestoreProgressUpdater struct {
	PodVolumeRestore *velerov1api.PodVolumeRestore
	Log              logrus.FieldLogger
	Ctx              context.Context
	Cli              client.Client
}

// +kubebuilder:rbac:groups=velero.io,resources=podvolumerestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=podvolumerestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumerclaims,verbs=get

func (c *PodVolumeRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.logger.WithField("PodVolumeRestore", req.NamespacedName.String())

	pvr := &velerov1api.PodVolumeRestore{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, pvr); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warn("PodVolumeRestore not found, skip")
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("Unable to get the PodVolumeRestore")
		return ctrl.Result{}, err
	}
	log = log.WithField("pod", fmt.Sprintf("%s/%s", pvr.Spec.Pod.Namespace, pvr.Spec.Pod.Name))
	if len(pvr.OwnerReferences) == 1 {
		log = log.WithField("restore", fmt.Sprintf("%s/%s", pvr.Namespace, pvr.OwnerReferences[0].Name))
	}

	shouldProcess, pod, err := c.shouldProcess(ctx, log, pvr)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !shouldProcess {
		return ctrl.Result{}, nil
	}

	initContainerIndex := getInitContainerIndex(pod)
	if initContainerIndex > 0 {
		log.Warnf(`Init containers before the %s container may cause issues
		          if they interfere with volumes being restored: %s index %d`, restorehelper.WaitInitContainer, restorehelper.WaitInitContainer, initContainerIndex)
	}

	log.Info("Restore starting")
	original := pvr.DeepCopy()
	pvr.Status.Phase = velerov1api.PodVolumeRestorePhaseInProgress
	pvr.Status.StartTimestamp = &metav1.Time{Time: c.clock.Now()}
	if err = c.Patch(ctx, pvr, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("Unable to update status to in progress")
		return ctrl.Result{}, err
	}

	if err = c.processRestore(ctx, pvr, pod, log); err != nil {
		if e := UpdatePVRStatusToFailed(c, ctx, pvr, err.Error(), c.clock.Now()); e != nil {
			log.WithError(err).Error("Unable to update status to failed")
		}

		log.WithError(err).Error("Unable to process the PodVolumeRestore")
		return ctrl.Result{}, err
	}

	original = pvr.DeepCopy()
	pvr.Status.Phase = velerov1api.PodVolumeRestorePhaseCompleted
	pvr.Status.CompletionTimestamp = &metav1.Time{Time: c.clock.Now()}
	if err = c.Patch(ctx, pvr, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("Unable to update status to completed")
		return ctrl.Result{}, err
	}
	log.Info("Restore completed")
	return ctrl.Result{}, nil
}

func UpdatePVRStatusToFailed(c client.Client, ctx context.Context, pvr *velerov1api.PodVolumeRestore, errString string, time time.Time) error {
	original := pvr.DeepCopy()
	pvr.Status.Phase = velerov1api.PodVolumeRestorePhaseFailed
	pvr.Status.Message = errString
	pvr.Status.CompletionTimestamp = &metav1.Time{Time: time}

	return c.Patch(ctx, pvr, client.MergeFrom(original))
}

func (c *PodVolumeRestoreReconciler) shouldProcess(ctx context.Context, log logrus.FieldLogger, pvr *velerov1api.PodVolumeRestore) (bool, *corev1api.Pod, error) {
	if !isPVRNew(pvr) {
		log.Debug("PodVolumeRestore is not new, skip")
		return false, nil, nil
	}

	// we filter the pods during the initialization of cache, if we can get a pod here, the pod must be in the same node with the controller
	// so we don't need to compare the node anymore
	pod := &corev1api.Pod{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: pvr.Spec.Pod.Namespace, Name: pvr.Spec.Pod.Name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debug("Pod not found on this node, skip")
			return false, nil, nil
		}
		log.WithError(err).Error("Unable to get pod")
		return false, nil, err
	}

	if !isInitContainerRunning(pod) {
		log.Debug("Pod is not running restore-wait init container, skip")
		return false, nil, nil
	}

	return true, pod, nil
}

func (c *PodVolumeRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The pod may not being scheduled at the point when its PVRs are initially reconciled.
	// By watching the pods, we can trigger the PVR reconciliation again once the pod is finally scheduled on the node.
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.PodVolumeRestore{}).
		Watches(&source.Kind{Type: &corev1api.Pod{}}, handler.EnqueueRequestsFromMapFunc(c.findVolumeRestoresForPod)).
		Complete(c)
}

func (c *PodVolumeRestoreReconciler) findVolumeRestoresForPod(pod client.Object) []reconcile.Request {
	list := &velerov1api.PodVolumeRestoreList{}
	options := &client.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			velerov1api.PodUIDLabel: string(pod.GetUID()),
		}).AsSelector(),
	}
	if err := c.List(context.TODO(), list, options); err != nil {
		c.logger.WithField("pod", fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())).WithError(err).
			Error("unable to list PodVolumeRestores")
		return []reconcile.Request{}
	}
	requests := make([]reconcile.Request, len(list.Items))
	for i, item := range list.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
		}
	}
	return requests
}

func isPVRNew(pvr *velerov1api.PodVolumeRestore) bool {
	return pvr.Status.Phase == "" || pvr.Status.Phase == velerov1api.PodVolumeRestorePhaseNew
}

func isInitContainerRunning(pod *corev1api.Pod) bool {
	// Pod volume wait container can be anywhere in the list of init containers, but must be running.
	i := getInitContainerIndex(pod)
	return i >= 0 &&
		len(pod.Status.InitContainerStatuses)-1 >= i &&
		pod.Status.InitContainerStatuses[i].State.Running != nil
}

func getInitContainerIndex(pod *corev1api.Pod) int {
	// Pod volume wait container can be anywhere in the list of init containers so locate it.
	for i, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == restorehelper.WaitInitContainer {
			return i
		}
	}

	return -1
}

func (c *PodVolumeRestoreReconciler) processRestore(ctx context.Context, req *velerov1api.PodVolumeRestore, pod *corev1api.Pod, log logrus.FieldLogger) error {
	volumeDir, err := kube.GetVolumeDirectory(ctx, log, pod, req.Spec.Volume, c.Client)
	if err != nil {
		return errors.Wrap(err, "error getting volume directory name")
	}

	// Get the full path of the new volume's directory as mounted in the daemonset pod, which
	// will look like: /host_pods/<new-pod-uid>/volumes/<volume-plugin-name>/<volume-dir>
	volumePath, err := kube.SinglePathMatch(
		fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(req.Spec.Pod.UID), volumeDir),
		c.fileSystem, log)
	if err != nil {
		return errors.Wrap(err, "error identifying path of volume")
	}

	backupLocation := &velerov1api.BackupStorageLocation{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Spec.BackupStorageLocation,
	}, backupLocation); err != nil {
		return errors.Wrap(err, "error getting backup storage location")
	}

	// need to check backup repository in source namespace rather than in pod namespace
	// such as in case of namespace mapping issue
	backupRepo, err := repository.GetBackupRepository(ctx, c.Client, req.Namespace, repository.BackupRepositoryKey{
		VolumeNamespace: req.Spec.SourceNamespace,
		BackupLocation:  req.Spec.BackupStorageLocation,
		RepositoryType:  podvolume.GetPvrRepositoryType(req),
	})
	if err != nil {
		return errors.Wrap(err, "error getting backup repository")
	}

	uploaderProv, err := provider.NewUploaderProvider(ctx, c.Client, req.Spec.UploaderType,
		req.Spec.RepoIdentifier, backupLocation, backupRepo, c.credentialGetter, repokey.RepoKeySelector(), log)
	if err != nil {
		return errors.Wrap(err, "error creating uploader")
	}

	defer func() {
		if err := uploaderProv.Close(ctx); err != nil {
			log.Errorf("failed to close uploader provider with error %v", err)
		}
	}()

	if err = uploaderProv.RunRestore(ctx, req.Spec.SnapshotID, volumePath, c.NewRestoreProgressUpdater(req, log, ctx)); err != nil {
		return errors.Wrapf(err, "error running restore err=%v", err)
	}

	// Remove the .velero directory from the restored volume (it may contain done files from previous restores
	// of this volume, which we don't want to carry over). If this fails for any reason, log and continue, since
	// this is non-essential cleanup (the done files are named based on restore UID and the init container looks
	// for the one specific to the restore being executed).
	if err := os.RemoveAll(filepath.Join(volumePath, ".velero")); err != nil {
		log.WithError(err).Warnf("error removing .velero directory from directory %s", volumePath)
	}

	var restoreUID types.UID
	for _, owner := range req.OwnerReferences {
		if boolptr.IsSetToTrue(owner.Controller) {
			restoreUID = owner.UID
			break
		}
	}

	// Create the .velero directory within the volume dir so we can write a done file
	// for this restore.
	if err := os.MkdirAll(filepath.Join(volumePath, ".velero"), 0755); err != nil {
		return errors.Wrap(err, "error creating .velero directory for done file")
	}

	// Write a done file with name=<restore-uid> into the just-created .velero dir
	// within the volume. The velero init container on the pod is waiting
	// for this file to exist in each restored volume before completing.
	if err := os.WriteFile(filepath.Join(volumePath, ".velero", string(restoreUID)), nil, 0644); err != nil { //nolint:gosec
		return errors.Wrap(err, "error writing done file")
	}

	return nil
}

func (r *PodVolumeRestoreReconciler) NewRestoreProgressUpdater(pvr *velerov1api.PodVolumeRestore, log logrus.FieldLogger, ctx context.Context) *RestoreProgressUpdater {
	return &RestoreProgressUpdater{pvr, log, ctx, r.Client}
}

// UpdateProgress which implement ProgressUpdater interface to update pvr progress status
func (r *RestoreProgressUpdater) UpdateProgress(p *uploader.UploaderProgress) {
	original := r.PodVolumeRestore.DeepCopy()
	r.PodVolumeRestore.Status.Progress = velerov1api.PodVolumeOperationProgress{TotalBytes: p.TotalBytes, BytesDone: p.BytesDone}
	if r.Cli == nil {
		r.Log.Errorf("failed to update restore pod %s volume %s progress with uninitailize client", r.PodVolumeRestore.Spec.Pod.Name, r.PodVolumeRestore.Spec.Volume)
		return
	}
	if err := r.Cli.Patch(r.Ctx, r.PodVolumeRestore, client.MergeFrom(original)); err != nil {
		r.Log.Errorf("update restore pod %s volume %s progress with %v", r.PodVolumeRestore.Spec.Pod.Name, r.PodVolumeRestore.Spec.Volume, err)
	}
}
