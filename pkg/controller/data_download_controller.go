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
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/vmware-tanzu/velero/internal/credentials"
	"github.com/vmware-tanzu/velero/pkg/apis/velero/shared"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	datamover "github.com/vmware-tanzu/velero/pkg/datamover"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	repository "github.com/vmware-tanzu/velero/pkg/repository"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// DataDownloadReconciler reconciles a DataDownload object
type DataDownloadReconciler struct {
	client            client.Client
	kubeClient        kubernetes.Interface
	logger            logrus.FieldLogger
	credentialGetter  *credentials.CredentialGetter
	fileSystem        filesystem.Interface
	Clock             clock.WithTickerAndDelayedExecution
	restoreExposer    exposer.GenericRestoreExposer
	nodeName          string
	repositoryEnsurer *repository.Ensurer
	dataPathMgr       *datapath.Manager
	preparingTimeout  time.Duration
	metrics           *metrics.ServerMetrics
}

func NewDataDownloadReconciler(client client.Client, kubeClient kubernetes.Interface,
	repoEnsurer *repository.Ensurer, credentialGetter *credentials.CredentialGetter, nodeName string, preparingTimeout time.Duration, logger logrus.FieldLogger, metrics *metrics.ServerMetrics) *DataDownloadReconciler {
	return &DataDownloadReconciler{
		client:            client,
		kubeClient:        kubeClient,
		logger:            logger.WithField("controller", "DataDownload"),
		credentialGetter:  credentialGetter,
		fileSystem:        filesystem.NewFileSystem(),
		Clock:             &clock.RealClock{},
		nodeName:          nodeName,
		repositoryEnsurer: repoEnsurer,
		restoreExposer:    exposer.NewGenericRestoreExposer(kubeClient, logger),
		dataPathMgr:       datapath.NewManager(1),
		preparingTimeout:  preparingTimeout,
		metrics:           metrics,
	}
}

// +kubebuilder:rbac:groups=velero.io,resources=datadownloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=datadownloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumerclaims,verbs=get

func (r *DataDownloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithFields(logrus.Fields{
		"controller":   "datadownload",
		"datadownload": req.NamespacedName,
	})

	log.Infof("Reconcile %s", req.Name)

	dd := &velerov2alpha1api.DataDownload{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, dd); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warn("DataDownload not found, skip")
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("Unable to get the DataDownload")
		return ctrl.Result{}, err
	}

	if !datamover.IsBuiltInUploader(dd.Spec.DataMover) {
		log.WithField("data mover", dd.Spec.DataMover).Info("it is not one built-in data mover which is not supported by Velero")
		return ctrl.Result{}, nil
	}

	if r.restoreExposer == nil {
		return r.errorOut(ctx, dd, errors.New("uninitialized generic exposer"), "uninitialized exposer", log)
	}

	if dd.Status.Phase == "" || dd.Status.Phase == velerov2alpha1api.DataDownloadPhaseNew {
		log.Info("Data download starting")

		if _, err := r.getTargetPVC(ctx, dd); err != nil {
			return ctrl.Result{Requeue: true}, nil
		}

		accepted, err := r.acceptDataDownload(ctx, dd)
		if err != nil {
			return r.errorOut(ctx, dd, err, "error to accept the data download", log)
		}

		if !accepted {
			log.Debug("Data download is not accepted")
			return ctrl.Result{}, nil
		}

		log.Info("Data download is accepted")

		if dd.Spec.Cancel {
			log.Debugf("Data download is been canceled %s in Phase %s", dd.GetName(), dd.Status.Phase)
			r.OnDataDownloadCancelled(ctx, dd.GetNamespace(), dd.GetName())
			return ctrl.Result{}, nil
		}

		hostingPodLabels := map[string]string{velerov1api.DataDownloadLabel: dd.Name}

		// Expose() will trigger to create one pod whose volume is restored by a given volume snapshot,
		// but the pod maybe is not in the same node of the current controller, so we need to return it here.
		// And then only the controller who is in the same node could do the rest work.
		err = r.restoreExposer.Expose(ctx, getDataDownloadOwnerObject(dd), dd.Spec.TargetVolume.PVC, dd.Spec.TargetVolume.Namespace, hostingPodLabels, dd.Spec.OperationTimeout.Duration)
		if err != nil {
			return r.errorOut(ctx, dd, err, "error to start restore expose", log)
		}
		log.Info("Restore is exposed")

		return ctrl.Result{}, nil
	} else if dd.Status.Phase == velerov2alpha1api.DataDownloadPhaseAccepted {
		if dd.Spec.Cancel {
			log.Debugf("Data download is been canceled %s in Phase %s", dd.GetName(), dd.Status.Phase)
			r.OnDataDownloadCancelled(ctx, dd.GetNamespace(), dd.GetName())
		} else if dd.Status.StartTimestamp != nil {
			if time.Since(dd.Status.StartTimestamp.Time) >= r.preparingTimeout {
				r.onPrepareTimeout(ctx, dd)
			}
		}

		return ctrl.Result{}, nil
	} else if dd.Status.Phase == velerov2alpha1api.DataDownloadPhasePrepared {
		log.Info("Data download is prepared")

		if dd.Spec.Cancel {
			log.Debugf("Data download is been canceled %s in Phase %s", dd.GetName(), dd.Status.Phase)
			r.OnDataDownloadCancelled(ctx, dd.GetNamespace(), dd.GetName())
			return ctrl.Result{}, nil
		}

		fsRestore := r.dataPathMgr.GetAsyncBR(dd.Name)

		if fsRestore != nil {
			log.Info("Cancellable data path is already started")
			return ctrl.Result{}, nil
		}

		result, err := r.restoreExposer.GetExposed(ctx, getDataDownloadOwnerObject(dd), r.client, r.nodeName, dd.Spec.OperationTimeout.Duration)
		if err != nil {
			return r.errorOut(ctx, dd, err, "restore exposer is not ready", log)
		} else if result == nil {
			log.Debug("Get empty restore exposer")
			return ctrl.Result{}, nil
		}

		log.Info("Restore PVC is ready and creating data path routine")

		// Need to first create file system BR and get data path instance then update data upload status
		callbacks := datapath.Callbacks{
			OnCompleted: r.OnDataDownloadCompleted,
			OnFailed:    r.OnDataDownloadFailed,
			OnCancelled: r.OnDataDownloadCancelled,
			OnProgress:  r.OnDataDownloadProgress,
		}

		fsRestore, err = r.dataPathMgr.CreateFileSystemBR(dd.Name, dataUploadDownloadRequestor, ctx, r.client, dd.Namespace, callbacks, log)
		if err != nil {
			if err == datapath.ConcurrentLimitExceed {
				log.Info("Data path instance is concurrent limited requeue later")
				return ctrl.Result{Requeue: true, RequeueAfter: time.Minute}, nil
			} else {
				return r.errorOut(ctx, dd, err, "error to create data path", log)
			}
		}

		// Update status to InProgress
		original := dd.DeepCopy()
		dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseInProgress
		if err := r.client.Patch(ctx, dd, client.MergeFrom(original)); err != nil {
			log.WithError(err).Error("Unable to update status to in progress")
			return ctrl.Result{}, err
		}

		log.Info("Data download is marked as in progress")

		reconcileResult, err := r.runCancelableDataPath(ctx, fsRestore, dd, result, log)
		if err != nil {
			log.Errorf("Failed to run cancelable data path for %s with err %v", dd.Name, err)
			r.closeDataPath(ctx, dd.Name)
		}
		return reconcileResult, err
	} else if dd.Status.Phase == velerov2alpha1api.DataDownloadPhaseInProgress {
		log.Info("Data download is in progress")
		if dd.Spec.Cancel {
			fsRestore := r.dataPathMgr.GetAsyncBR(dd.Name)
			if fsRestore == nil {
				return ctrl.Result{}, nil
			}

			log.Info("Data download is being canceled")
			// Update status to Canceling.
			original := dd.DeepCopy()
			dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseCanceling
			if err := r.client.Patch(ctx, dd, client.MergeFrom(original)); err != nil {
				log.WithError(err).Error("error updating data download status")
				return ctrl.Result{}, err
			}

			fsRestore.Cancel()
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, nil
	} else {
		log.Debugf("Data download now is in %s phase and do nothing by current %s controller", dd.Status.Phase, r.nodeName)
		return ctrl.Result{}, nil
	}
}

func (r *DataDownloadReconciler) runCancelableDataPath(ctx context.Context, fsRestore datapath.AsyncBR, dd *velerov2alpha1api.DataDownload, res *exposer.ExposeResult, log logrus.FieldLogger) (reconcile.Result, error) {
	path, err := exposer.GetPodVolumeHostPath(ctx, res.ByPod.HostingPod, res.ByPod.VolumeName, r.client, r.fileSystem, log)
	if err != nil {
		return r.errorOut(ctx, dd, err, "error exposing host path for pod volume", log)
	}

	log.WithField("path", path.ByPath).Debug("Found host path")
	if err := fsRestore.Init(ctx, dd.Spec.BackupStorageLocation, dd.Spec.SourceNamespace, datamover.GetUploaderType(dd.Spec.DataMover),
		velerov1api.BackupRepositoryTypeKopia, "", r.repositoryEnsurer, r.credentialGetter); err != nil {
		return r.errorOut(ctx, dd, err, "error to initialize data path", log)
	}
	log.WithField("path", path.ByPath).Info("fs init")

	if err := fsRestore.StartRestore(dd.Spec.SnapshotID, path); err != nil {
		return r.errorOut(ctx, dd, err, fmt.Sprintf("error starting data path %s restore", path.ByPath), log)
	}

	log.WithField("path", path.ByPath).Info("Async fs restore data path started")
	return ctrl.Result{}, nil
}

func (r *DataDownloadReconciler) OnDataDownloadCompleted(ctx context.Context, namespace string, ddName string, result datapath.Result) {
	defer r.closeDataPath(ctx, ddName)

	log := r.logger.WithField("datadownload", ddName)
	log.Info("Async fs restore data path completed")

	var dd velerov2alpha1api.DataDownload
	if err := r.client.Get(ctx, types.NamespacedName{Name: ddName, Namespace: namespace}, &dd); err != nil {
		log.WithError(err).Warn("Failed to get datadownload on completion")
		return
	}

	objRef := getDataDownloadOwnerObject(&dd)
	err := r.restoreExposer.RebindVolume(ctx, objRef, dd.Spec.TargetVolume.PVC, dd.Spec.TargetVolume.Namespace, dd.Spec.OperationTimeout.Duration)
	if err != nil {
		log.WithError(err).Error("Failed to rebind PV to target PVC on completion")
		return
	}

	log.Info("Cleaning up exposed environment")
	r.restoreExposer.CleanUp(ctx, objRef)

	original := dd.DeepCopy()
	dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseCompleted
	dd.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
	if err := r.client.Patch(ctx, &dd, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("error updating data download status")
	} else {
		log.Infof("Data download is marked as %s", dd.Status.Phase)
		r.metrics.RegisterDataDownloadSuccess(r.nodeName)
	}
}

func (r *DataDownloadReconciler) OnDataDownloadFailed(ctx context.Context, namespace string, ddName string, err error) {
	defer r.closeDataPath(ctx, ddName)

	log := r.logger.WithField("datadownload", ddName)

	log.WithError(err).Error("Async fs restore data path failed")

	var dd velerov2alpha1api.DataDownload
	if getErr := r.client.Get(ctx, types.NamespacedName{Name: ddName, Namespace: namespace}, &dd); getErr != nil {
		log.WithError(getErr).Warn("Failed to get data download on failure")
	} else {
		if _, errOut := r.errorOut(ctx, &dd, err, "data path restore failed", log); err != nil {
			log.WithError(err).Warnf("Failed to patch data download with err %v", errOut)
		}
	}
}

func (r *DataDownloadReconciler) OnDataDownloadCancelled(ctx context.Context, namespace string, ddName string) {
	defer r.closeDataPath(ctx, ddName)

	log := r.logger.WithField("datadownload", ddName)

	log.Warn("Async fs backup data path canceled")

	var dd velerov2alpha1api.DataDownload
	if getErr := r.client.Get(ctx, types.NamespacedName{Name: ddName, Namespace: namespace}, &dd); getErr != nil {
		log.WithError(getErr).Warn("Failed to get datadownload on cancel")
	} else {
		// cleans up any objects generated during the snapshot expose
		r.restoreExposer.CleanUp(ctx, getDataDownloadOwnerObject(&dd))

		original := dd.DeepCopy()
		dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseCanceled
		if dd.Status.StartTimestamp.IsZero() {
			dd.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
		}
		dd.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
		if err := r.client.Patch(ctx, &dd, client.MergeFrom(original)); err != nil {
			log.WithError(err).Error("error updating data download status")
		} else {
			r.metrics.RegisterDataDownloadCancel(r.nodeName)
		}
	}
}

func (r *DataDownloadReconciler) OnDataDownloadProgress(ctx context.Context, namespace string, ddName string, progress *uploader.Progress) {
	log := r.logger.WithField("datadownload", ddName)

	var dd velerov2alpha1api.DataDownload
	if err := r.client.Get(ctx, types.NamespacedName{Name: ddName, Namespace: namespace}, &dd); err != nil {
		log.WithError(err).Warn("Failed to get data download on progress")
		return
	}

	original := dd.DeepCopy()
	dd.Status.Progress = shared.DataMoveOperationProgress{TotalBytes: progress.TotalBytes, BytesDone: progress.BytesDone}

	if err := r.client.Patch(ctx, &dd, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("Failed to update restore snapshot progress")
	}
}

// SetupWithManager registers the DataDownload controller.
// The fresh new DataDownload CR first created will trigger to create one pod (long time, maybe failure or unknown status) by one of the datadownload controllers
// then the request will get out of the Reconcile queue immediately by not blocking others' CR handling, in order to finish the rest data download process we need to
// re-enqueue the previous related request once the related pod is in running status to keep going on the rest logic. and below logic will avoid handling the unwanted
// pod status and also avoid block others CR handling
func (r *DataDownloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(r.logger, r.client, &velerov2alpha1api.DataDownloadList{}, preparingMonitorFrequency, kube.PeriodicalEnqueueSourceOption{})
	gp := kube.NewGenericEventPredicate(func(object client.Object) bool {
		dd := object.(*velerov2alpha1api.DataDownload)
		return (dd.Status.Phase == velerov2alpha1api.DataDownloadPhaseAccepted)
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov2alpha1api.DataDownload{}).
		Watches(s, nil, builder.WithPredicates(gp)).
		Watches(&source.Kind{Type: &v1.Pod{}}, kube.EnqueueRequestsFromMapUpdateFunc(r.findSnapshotRestoreForPod),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(ue event.UpdateEvent) bool {
					newObj := ue.ObjectNew.(*v1.Pod)

					if _, ok := newObj.Labels[velerov1api.DataDownloadLabel]; !ok {
						return false
					}

					if newObj.Status.Phase != v1.PodRunning {
						return false
					}

					if newObj.Spec.NodeName == "" {
						return false
					}

					return true
				},
				CreateFunc: func(event.CreateEvent) bool {
					return false
				},
				DeleteFunc: func(de event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(ge event.GenericEvent) bool {
					return false
				},
			})).
		Complete(r)
}

func (r *DataDownloadReconciler) findSnapshotRestoreForPod(podObj client.Object) []reconcile.Request {
	pod := podObj.(*v1.Pod)

	dd, err := findDataDownloadByPod(r.client, *pod)
	if err != nil {
		r.logger.WithField("Restore pod", pod.Name).WithError(err).Error("unable to get DataDownload")
		return []reconcile.Request{}
	} else if dd == nil {
		r.logger.WithField("Restore pod", pod.Name).Error("get empty DataDownload")
		return []reconcile.Request{}
	}

	if dd.Status.Phase != velerov2alpha1api.DataDownloadPhaseAccepted {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 1)

	r.logger.WithField("Restore pod", pod.Name).Infof("Preparing data download %s", dd.Name)

	// we don't expect anyone else update the CR during the Prepare process
	updated, err := r.exclusiveUpdateDataDownload(context.Background(), dd, r.prepareDataDownload)
	if err != nil || !updated {
		r.logger.WithFields(logrus.Fields{
			"Datadownload": dd.Name,
			"Restore pod":  pod.Name,
			"updated":      updated,
		}).WithError(err).Warn("failed to patch datadownload, prepare will halt for this datadownload")
		return []reconcile.Request{}
	}

	requests[0] = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: dd.Namespace,
			Name:      dd.Name,
		},
	}

	return requests
}

func (r *DataDownloadReconciler) FindDataDownloads(ctx context.Context, cli client.Client, ns string) ([]*velerov2alpha1api.DataDownload, error) {
	pods := &v1.PodList{}
	var dataDownloads []*velerov2alpha1api.DataDownload
	if err := cli.List(ctx, pods, &client.ListOptions{Namespace: ns}); err != nil {
		r.logger.WithError(errors.WithStack(err)).Error("failed to list pods on current node")
		return nil, errors.Wrapf(err, "failed to list pods on current node")
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName != r.nodeName {
			r.logger.Debugf("Pod %s related data download will not handled by %s nodes", pod.GetName(), r.nodeName)
			continue
		}
		dd, err := findDataDownloadByPod(cli, pod)
		if err != nil {
			r.logger.WithError(errors.WithStack(err)).Error("failed to get dataDownload by pod")
			continue
		} else if dd != nil {
			dataDownloads = append(dataDownloads, dd)
		}
	}
	return dataDownloads, nil
}

func (r *DataDownloadReconciler) prepareDataDownload(ssb *velerov2alpha1api.DataDownload) {
	ssb.Status.Phase = velerov2alpha1api.DataDownloadPhasePrepared
	ssb.Status.Node = r.nodeName
}

func (r *DataDownloadReconciler) errorOut(ctx context.Context, dd *velerov2alpha1api.DataDownload, err error, msg string, log logrus.FieldLogger) (ctrl.Result, error) {
	if r.restoreExposer != nil {
		r.restoreExposer.CleanUp(ctx, getDataDownloadOwnerObject(dd))
	}
	return ctrl.Result{}, r.updateStatusToFailed(ctx, dd, err, msg, log)
}

func (r *DataDownloadReconciler) updateStatusToFailed(ctx context.Context, dd *velerov2alpha1api.DataDownload, err error, msg string, log logrus.FieldLogger) error {
	log.Infof("update data download status to %v", dd.Status.Phase)
	original := dd.DeepCopy()
	dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseFailed
	dd.Status.Message = errors.WithMessage(err, msg).Error()
	dd.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}

	if patchErr := r.client.Patch(ctx, dd, client.MergeFrom(original)); patchErr != nil {
		log.WithError(patchErr).Error("error updating DataDownload status")
	} else {
		r.metrics.RegisterDataDownloadFailure(r.nodeName)
	}

	return err
}

func (r *DataDownloadReconciler) acceptDataDownload(ctx context.Context, dd *velerov2alpha1api.DataDownload) (bool, error) {
	r.logger.Infof("Accepting data download %s", dd.Name)

	// For all data download controller in each node-agent will try to update download CR, and only one controller will success,
	// and the success one could handle later logic
	succeeded, err := r.exclusiveUpdateDataDownload(ctx, dd, func(dd *velerov2alpha1api.DataDownload) {
		dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseAccepted
		dd.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
	})

	if err != nil {
		return false, err
	}

	if succeeded {
		r.logger.WithField("DataDownload", dd.Name).Infof("This datadownload has been accepted by %s", r.nodeName)
		return true, nil
	}

	r.logger.WithField("DataDownload", dd.Name).Info("This datadownload has been accepted by others")
	return false, nil
}

func (r *DataDownloadReconciler) onPrepareTimeout(ctx context.Context, dd *velerov2alpha1api.DataDownload) {
	log := r.logger.WithField("DataDownload", dd.Name)

	log.Info("Timeout happened for preparing datadownload")

	succeeded, err := r.exclusiveUpdateDataDownload(ctx, dd, func(dd *velerov2alpha1api.DataDownload) {
		dd.Status.Phase = velerov2alpha1api.DataDownloadPhaseFailed
		dd.Status.Message = "timeout on preparing data download"
	})

	if err != nil {
		log.WithError(err).Warn("Failed to update datadownload")
		return
	}

	if !succeeded {
		log.Warn("Dataupload has been updated by others")
		return
	}

	r.restoreExposer.CleanUp(ctx, getDataDownloadOwnerObject(dd))

	log.Info("Dataupload has been cleaned up")

	r.metrics.RegisterDataDownloadFailure(r.nodeName)
}

func (r *DataDownloadReconciler) exclusiveUpdateDataDownload(ctx context.Context, dd *velerov2alpha1api.DataDownload,
	updateFunc func(*velerov2alpha1api.DataDownload)) (bool, error) {
	updated := dd.DeepCopy()
	updateFunc(updated)

	err := r.client.Update(ctx, updated)
	if err == nil {
		return true, nil
	} else if apierrors.IsConflict(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (r *DataDownloadReconciler) getTargetPVC(ctx context.Context, dd *velerov2alpha1api.DataDownload) (*v1.PersistentVolumeClaim, error) {
	return r.kubeClient.CoreV1().PersistentVolumeClaims(dd.Spec.TargetVolume.Namespace).Get(ctx, dd.Spec.TargetVolume.PVC, metav1.GetOptions{})
}

func (r *DataDownloadReconciler) closeDataPath(ctx context.Context, ddName string) {
	fsBackup := r.dataPathMgr.GetAsyncBR(ddName)
	if fsBackup != nil {
		fsBackup.Close(ctx)
	}

	r.dataPathMgr.RemoveAsyncBR(ddName)
}

func getDataDownloadOwnerObject(dd *velerov2alpha1api.DataDownload) v1.ObjectReference {
	return v1.ObjectReference{
		Kind:       dd.Kind,
		Namespace:  dd.Namespace,
		Name:       dd.Name,
		UID:        dd.UID,
		APIVersion: dd.APIVersion,
	}
}

func findDataDownloadByPod(client client.Client, pod v1.Pod) (*velerov2alpha1api.DataDownload, error) {
	if label, exist := pod.Labels[velerov1api.DataDownloadLabel]; exist {
		dd := &velerov2alpha1api.DataDownload{}
		err := client.Get(context.Background(), types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      label,
		}, dd)

		if err != nil {
			return nil, errors.Wrapf(err, "error to find DataDownload by pod %s/%s", pod.Namespace, pod.Name)
		}
		return dd, nil
	}

	return nil, nil
}
