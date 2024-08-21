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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotter "github.com/kubernetes-csi/external-snapshotter/client/v7/clientset/versioned/typed/volumesnapshot/v1"

	"github.com/vmware-tanzu/velero/pkg/apis/velero/shared"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/datamover"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	dataUploadDownloadRequestor = "snapshot-data-upload-download"
	acceptNodeLabelKey          = "velero.io/accepted-by"
	DataUploadDownloadFinalizer = "velero.io/data-upload-download-finalizer"
	preparingMonitorFrequency   = time.Minute
)

// DataUploadReconciler reconciles a DataUpload object
type DataUploadReconciler struct {
	client              client.Client
	kubeClient          kubernetes.Interface
	csiSnapshotClient   snapshotter.SnapshotV1Interface
	mgr                 manager.Manager
	Clock               clocks.WithTickerAndDelayedExecution
	nodeName            string
	logger              logrus.FieldLogger
	snapshotExposerList map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer
	dataPathMgr         *datapath.Manager
	loadAffinity        *nodeagent.LoadAffinity
	backupPVCConfig     map[string]nodeagent.BackupPVC
	podResources        corev1.ResourceRequirements
	preparingTimeout    time.Duration
	metrics             *metrics.ServerMetrics
}

func NewDataUploadReconciler(client client.Client, mgr manager.Manager, kubeClient kubernetes.Interface, csiSnapshotClient snapshotter.SnapshotV1Interface,
	dataPathMgr *datapath.Manager, loadAffinity *nodeagent.LoadAffinity, backupPVCConfig map[string]nodeagent.BackupPVC, podResources corev1.ResourceRequirements,
	clock clocks.WithTickerAndDelayedExecution, nodeName string, preparingTimeout time.Duration, log logrus.FieldLogger, metrics *metrics.ServerMetrics) *DataUploadReconciler {
	return &DataUploadReconciler{
		client:              client,
		mgr:                 mgr,
		kubeClient:          kubeClient,
		csiSnapshotClient:   csiSnapshotClient,
		Clock:               clock,
		nodeName:            nodeName,
		logger:              log,
		snapshotExposerList: map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{velerov2alpha1api.SnapshotTypeCSI: exposer.NewCSISnapshotExposer(kubeClient, csiSnapshotClient, log)},
		dataPathMgr:         dataPathMgr,
		loadAffinity:        loadAffinity,
		backupPVCConfig:     backupPVCConfig,
		podResources:        podResources,
		preparingTimeout:    preparingTimeout,
		metrics:             metrics,
	}
}

// +kubebuilder:rbac:groups=velero.io,resources=datauploads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=datauploads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumerclaims,verbs=get

func (r *DataUploadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithFields(logrus.Fields{
		"controller": "dataupload",
		"dataupload": req.NamespacedName,
	})
	log.Infof("Reconcile %s", req.Name)
	du := &velerov2alpha1api.DataUpload{}
	if err := r.client.Get(ctx, req.NamespacedName, du); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find DataUpload")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "getting DataUpload")
	}

	if !datamover.IsBuiltInUploader(du.Spec.DataMover) {
		log.WithField("Data mover", du.Spec.DataMover).Debug("it is not one built-in data mover which is not supported by Velero")
		return ctrl.Result{}, nil
	}

	ep, ok := r.snapshotExposerList[du.Spec.SnapshotType]
	if !ok {
		return r.errorOut(ctx, du, errors.Errorf("%s type of snapshot exposer is not exist", du.Spec.SnapshotType), "not exist type of exposer", log)
	}

	// Logic for clear resources when dataupload been deleted
	if du.DeletionTimestamp.IsZero() { // add finalizer for all cr at beginning
		if !isDataUploadInFinalState(du) && !controllerutil.ContainsFinalizer(du, DataUploadDownloadFinalizer) {
			succeeded, err := r.exclusiveUpdateDataUpload(ctx, du, func(du *velerov2alpha1api.DataUpload) {
				controllerutil.AddFinalizer(du, DataUploadDownloadFinalizer)
			})

			if err != nil {
				log.Errorf("failed to add finalizer with error %s for %s/%s", err.Error(), du.Namespace, du.Name)
				return ctrl.Result{}, err
			} else if !succeeded {
				log.Warnf("failed to add finalizer for %s/%s and will requeue later", du.Namespace, du.Name)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	} else if controllerutil.ContainsFinalizer(du, DataUploadDownloadFinalizer) && !du.Spec.Cancel && !isDataUploadInFinalState(du) {
		// when delete cr we need to clear up internal resources created by Velero, here we use the cancel mechanism
		// to help clear up resources instead of clear them directly in case of some conflict with Expose action
		log.Warnf("Cancel du under phase %s because it is being deleted", du.Status.Phase)

		if err := UpdateDataUploadWithRetry(ctx, r.client, req.NamespacedName, log, func(dataUpload *velerov2alpha1api.DataUpload) bool {
			if dataUpload.Spec.Cancel {
				return false
			}

			dataUpload.Spec.Cancel = true
			dataUpload.Status.Message = "Cancel dataupload because it is being deleted"

			return true
		}); err != nil {
			log.Errorf("failed to set cancel flag with error %s for %s/%s", err.Error(), du.Namespace, du.Name)
			return ctrl.Result{}, err
		}
	}

	if du.Status.Phase == "" || du.Status.Phase == velerov2alpha1api.DataUploadPhaseNew {
		log.Info("Data upload starting")

		accepted, err := r.acceptDataUpload(ctx, du)
		if err != nil {
			return r.errorOut(ctx, du, err, "error to accept the data upload", log)
		}

		if !accepted {
			log.Debug("Data upload is not accepted")
			return ctrl.Result{}, nil
		}

		log.Info("Data upload is accepted")

		if du.Spec.Cancel {
			r.OnDataUploadCancelled(ctx, du.GetNamespace(), du.GetName())
			return ctrl.Result{}, nil
		}

		exposeParam, err := r.setupExposeParam(du)
		if err != nil {
			return r.errorOut(ctx, du, err, "failed to set exposer parameters", log)
		}

		// Expose() will trigger to create one pod whose volume is restored by a given volume snapshot,
		// but the pod maybe is not in the same node of the current controller, so we need to return it here.
		// And then only the controller who is in the same node could do the rest work.
		if err := ep.Expose(ctx, getOwnerObject(du), exposeParam); err != nil {
			if err := r.client.Get(ctx, req.NamespacedName, du); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, errors.Wrap(err, "getting DataUpload")
				}
			}
			if isDataUploadInFinalState(du) {
				log.Warnf("expose snapshot with err %v but it may caused by clean up resources in cancel action", err)
				r.cleanUp(ctx, du, log)
				return ctrl.Result{}, nil
			} else {
				return r.errorOut(ctx, du, err, "error to expose snapshot", log)
			}
		}

		log.Info("Snapshot is exposed")

		// we need to get CR again for it may canceled by dataupload controller on other
		// nodes when doing expose action, if detectd cancel action we need to clear up the internal
		// resources created by velero during backup.
		if err := r.client.Get(ctx, req.NamespacedName, du); err != nil {
			if apierrors.IsNotFound(err) {
				log.Debug("Unable to find DataUpload")
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, errors.Wrap(err, "getting DataUpload")
		}

		// we need to clean up resources as resources created in Expose it may later than cancel action or prepare time
		// and need to clean up resources again
		if isDataUploadInFinalState(du) {
			r.cleanUp(ctx, du, log)
		}

		return ctrl.Result{}, nil
	} else if du.Status.Phase == velerov2alpha1api.DataUploadPhaseAccepted {
		if du.Spec.Cancel {
			// we don't want to update CR into cancel status forcely as it may conflict with CR update in Expose action
			// we could retry when the CR requeue in periodcally
			log.Debugf("Data upload is been canceled %s in Phase %s", du.GetName(), du.Status.Phase)
			r.tryCancelAcceptedDataUpload(ctx, du, "")
		} else if peekErr := ep.PeekExposed(ctx, getOwnerObject(du)); peekErr != nil {
			r.tryCancelAcceptedDataUpload(ctx, du, fmt.Sprintf("found a dataupload %s/%s with expose error: %s. mark it as cancel", du.Namespace, du.Name, peekErr))
			log.Errorf("Cancel du %s/%s because of expose error %s", du.Namespace, du.Name, peekErr)
		} else if du.Status.StartTimestamp != nil {
			if time.Since(du.Status.StartTimestamp.Time) >= r.preparingTimeout {
				r.onPrepareTimeout(ctx, du)
			}
		}

		return ctrl.Result{}, nil
	} else if du.Status.Phase == velerov2alpha1api.DataUploadPhasePrepared {
		log.Info("Data upload is prepared")

		if du.Spec.Cancel {
			r.OnDataUploadCancelled(ctx, du.GetNamespace(), du.GetName())
			return ctrl.Result{}, nil
		}

		asyncBR := r.dataPathMgr.GetAsyncBR(du.Name)
		if asyncBR != nil {
			log.Info("Cancellable data path is already started")
			return ctrl.Result{}, nil
		}
		waitExposePara := r.setupWaitExposePara(du)
		res, err := ep.GetExposed(ctx, getOwnerObject(du), du.Spec.OperationTimeout.Duration, waitExposePara)
		if err != nil {
			return r.errorOut(ctx, du, err, "exposed snapshot is not ready", log)
		} else if res == nil {
			log.Debug("Get empty exposer")
			return ctrl.Result{}, nil
		}

		log.Info("Exposed snapshot is ready and creating data path routine")

		// Need to first create file system BR and get data path instance then update data upload status
		callbacks := datapath.Callbacks{
			OnCompleted: r.OnDataUploadCompleted,
			OnFailed:    r.OnDataUploadFailed,
			OnCancelled: r.OnDataUploadCancelled,
			OnProgress:  r.OnDataUploadProgress,
		}

		asyncBR, err = r.dataPathMgr.CreateMicroServiceBRWatcher(ctx, r.client, r.kubeClient, r.mgr, datapath.TaskTypeBackup,
			du.Name, du.Namespace, res.ByPod.HostingPod.Name, res.ByPod.HostingContainer, du.Name, callbacks, false, log)
		if err != nil {
			if err == datapath.ConcurrentLimitExceed {
				log.Info("Data path instance is concurrent limited requeue later")
				return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
			} else {
				return r.errorOut(ctx, du, err, "error to create data path", log)
			}
		}

		if err := r.initCancelableDataPath(ctx, asyncBR, res, log); err != nil {
			log.WithError(err).Errorf("Failed to init cancelable data path for %s", du.Name)

			r.closeDataPath(ctx, du.Name)
			return r.errorOut(ctx, du, err, "error initializing data path", log)
		}

		// Update status to InProgress
		original := du.DeepCopy()
		du.Status.Phase = velerov2alpha1api.DataUploadPhaseInProgress
		du.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
		if err := r.client.Patch(ctx, du, client.MergeFrom(original)); err != nil {
			log.WithError(err).Warnf("Failed to update dataupload %s to InProgress, will data path close and retry", du.Name)

			r.closeDataPath(ctx, du.Name)
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
		}

		log.Info("Data upload is marked as in progress")

		if err := r.startCancelableDataPath(asyncBR, du, res, log); err != nil {
			log.WithError(err).Errorf("Failed to start cancelable data path for %s", du.Name)
			r.closeDataPath(ctx, du.Name)

			return r.errorOut(ctx, du, err, "error starting data path", log)
		}

		return ctrl.Result{}, nil
	} else if du.Status.Phase == velerov2alpha1api.DataUploadPhaseInProgress {
		log.Info("Data upload is in progress")
		if du.Spec.Cancel {
			log.Info("Data upload is being canceled")

			asyncBR := r.dataPathMgr.GetAsyncBR(du.Name)
			if asyncBR == nil {
				if du.Status.Node == r.nodeName {
					r.OnDataUploadCancelled(ctx, du.GetNamespace(), du.GetName())
				} else {
					log.Info("Data path is not started in this node and will not canceled by current node")
				}
				return ctrl.Result{}, nil
			}

			// Update status to Canceling
			original := du.DeepCopy()
			du.Status.Phase = velerov2alpha1api.DataUploadPhaseCanceling
			if err := r.client.Patch(ctx, du, client.MergeFrom(original)); err != nil {
				log.WithError(err).Error("error updating data upload into canceling status")
				return ctrl.Result{}, err
			}
			asyncBR.Cancel()
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	} else {
		// put the finalizer remove action here for all cr will goes to the final status, we could check finalizer and do remove action in final status
		// instead of intermediate state.
		// remove finalizer no matter whether the cr is being deleted or not for it is no longer needed when internal resources are all cleaned up
		// also in final status cr won't block the direct delete of the velero namespace
		if isDataUploadInFinalState(du) && controllerutil.ContainsFinalizer(du, DataUploadDownloadFinalizer) {
			original := du.DeepCopy()
			controllerutil.RemoveFinalizer(du, DataUploadDownloadFinalizer)
			if err := r.client.Patch(ctx, du, client.MergeFrom(original)); err != nil {
				log.WithError(err).Error("error to remove finalizer")
			}
		}
		return ctrl.Result{}, nil
	}
}

func (r *DataUploadReconciler) initCancelableDataPath(ctx context.Context, asyncBR datapath.AsyncBR, res *exposer.ExposeResult, log logrus.FieldLogger) error {
	log.Info("Init cancelable dataUpload")

	if err := asyncBR.Init(ctx, nil); err != nil {
		return errors.Wrap(err, "error initializing asyncBR")
	}

	log.Infof("async backup init for pod %s, volume %s", res.ByPod.HostingPod.Name, res.ByPod.VolumeName)

	return nil
}

func (r *DataUploadReconciler) startCancelableDataPath(asyncBR datapath.AsyncBR, du *velerov2alpha1api.DataUpload, res *exposer.ExposeResult, log logrus.FieldLogger) error {
	log.Info("Start cancelable dataUpload")

	if err := asyncBR.StartBackup(datapath.AccessPoint{
		ByPath: res.ByPod.VolumeName,
	}, du.Spec.DataMoverConfig, nil); err != nil {
		return errors.Wrapf(err, "error starting async backup for pod %s, volume %s", res.ByPod.HostingPod.Name, res.ByPod.VolumeName)
	}

	log.Infof("Async backup started for pod %s, volume %s", res.ByPod.HostingPod.Name, res.ByPod.VolumeName)
	return nil
}

func (r *DataUploadReconciler) OnDataUploadCompleted(ctx context.Context, namespace string, duName string, result datapath.Result) {
	defer r.dataPathMgr.RemoveAsyncBR(duName)

	log := r.logger.WithField("dataupload", duName)

	log.Info("Async fs backup data path completed")

	var du velerov2alpha1api.DataUpload
	if err := r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, &du); err != nil {
		log.WithError(err).Warn("Failed to get dataupload on completion")
		return
	}

	// cleans up any objects generated during the snapshot expose
	ep, ok := r.snapshotExposerList[du.Spec.SnapshotType]
	if !ok {
		log.WithError(fmt.Errorf("%v type of snapshot exposer is not exist", du.Spec.SnapshotType)).
			Warn("Failed to clean up resources on completion")
	} else {
		var volumeSnapshotName string
		if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI { // Other exposer should have another condition
			volumeSnapshotName = du.Spec.CSISnapshot.VolumeSnapshot
		}
		ep.CleanUp(ctx, getOwnerObject(&du), volumeSnapshotName, du.Spec.SourceNamespace)
	}

	// Update status to Completed with path & snapshot ID.
	original := du.DeepCopy()
	du.Status.Path = result.Backup.Source.ByPath
	du.Status.Phase = velerov2alpha1api.DataUploadPhaseCompleted
	du.Status.SnapshotID = result.Backup.SnapshotID
	du.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
	if result.Backup.EmptySnapshot {
		du.Status.Message = "volume was empty so no data was upload"
	}

	if err := r.client.Patch(ctx, &du, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("error updating DataUpload status")
	} else {
		log.Info("Data upload completed")
		r.metrics.RegisterDataUploadSuccess(r.nodeName)
	}
}

func (r *DataUploadReconciler) OnDataUploadFailed(ctx context.Context, namespace, duName string, err error) {
	defer r.dataPathMgr.RemoveAsyncBR(duName)

	log := r.logger.WithField("dataupload", duName)

	log.WithError(err).Error("Async fs backup data path failed")

	var du velerov2alpha1api.DataUpload
	if getErr := r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, &du); getErr != nil {
		log.WithError(getErr).Warn("Failed to get dataupload on failure")
	} else {
		_, _ = r.errorOut(ctx, &du, err, "data path backup failed", log)
	}
}

func (r *DataUploadReconciler) OnDataUploadCancelled(ctx context.Context, namespace string, duName string) {
	defer r.dataPathMgr.RemoveAsyncBR(duName)

	log := r.logger.WithField("dataupload", duName)

	log.Warn("Async fs backup data path canceled")

	du := &velerov2alpha1api.DataUpload{}
	if getErr := r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, du); getErr != nil {
		log.WithError(getErr).Warn("Failed to get dataupload on cancel")
	} else {
		// cleans up any objects generated during the snapshot expose
		r.cleanUp(ctx, du, log)
		original := du.DeepCopy()
		du.Status.Phase = velerov2alpha1api.DataUploadPhaseCanceled
		if du.Status.StartTimestamp.IsZero() {
			du.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
		}
		du.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
		if err := r.client.Patch(ctx, du, client.MergeFrom(original)); err != nil {
			log.WithError(err).Error("error updating DataUpload status")
		} else {
			r.metrics.RegisterDataUploadCancel(r.nodeName)
		}
	}
}

func (r *DataUploadReconciler) tryCancelAcceptedDataUpload(ctx context.Context, du *velerov2alpha1api.DataUpload, message string) {
	log := r.logger.WithField("dataupload", du.Name)
	log.Warn("Accepted data upload is canceled")
	succeeded, err := r.exclusiveUpdateDataUpload(ctx, du, func(dataUpload *velerov2alpha1api.DataUpload) {
		dataUpload.Status.Phase = velerov2alpha1api.DataUploadPhaseCanceled
		if dataUpload.Status.StartTimestamp.IsZero() {
			dataUpload.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
		}
		dataUpload.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}

		if message != "" {
			dataUpload.Status.Message = message
		}
	})

	if err != nil {
		log.WithError(err).Error("error updating dataupload status")
		return
	} else if !succeeded {
		log.Warn("conflict in updating dataupload status and will try it again later")
		return
	}

	// success update
	r.metrics.RegisterDataUploadCancel(r.nodeName)
	// cleans up any objects generated during the snapshot expose
	r.cleanUp(ctx, du, log)
}

func (r *DataUploadReconciler) cleanUp(ctx context.Context, du *velerov2alpha1api.DataUpload, log logrus.FieldLogger) {
	ep, ok := r.snapshotExposerList[du.Spec.SnapshotType]
	if !ok {
		log.WithError(fmt.Errorf("%v type of snapshot exposer is not exist", du.Spec.SnapshotType)).
			Warn("Failed to clean up resources on canceled")
	} else {
		var volumeSnapshotName string
		if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI { // Other exposer should have another condition
			volumeSnapshotName = du.Spec.CSISnapshot.VolumeSnapshot
		}
		ep.CleanUp(ctx, getOwnerObject(du), volumeSnapshotName, du.Spec.SourceNamespace)
	}
}

func (r *DataUploadReconciler) OnDataUploadProgress(ctx context.Context, namespace string, duName string, progress *uploader.Progress) {
	log := r.logger.WithField("dataupload", duName)

	var du velerov2alpha1api.DataUpload
	if err := r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, &du); err != nil {
		log.WithError(err).Warn("Failed to get dataupload on progress")
		return
	}

	original := du.DeepCopy()
	du.Status.Progress = shared.DataMoveOperationProgress{TotalBytes: progress.TotalBytes, BytesDone: progress.BytesDone}

	if err := r.client.Patch(ctx, &du, client.MergeFrom(original)); err != nil {
		log.WithError(err).Error("Failed to update progress")
	}
}

// SetupWithManager registers the DataUpload controller.
// The fresh new DataUpload CR first created will trigger to create one pod (long time, maybe failure or unknown status) by one of the dataupload controllers
// then the request will get out of the Reconcile queue immediately by not blocking others' CR handling, in order to finish the rest data upload process we need to
// re-enqueue the previous related request once the related pod is in running status to keep going on the rest logic. and below logic will avoid handling the unwanted
// pod status and also avoid block others CR handling
func (r *DataUploadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(r.logger, r.client, &velerov2alpha1api.DataUploadList{}, preparingMonitorFrequency, kube.PeriodicalEnqueueSourceOption{})
	gp := kube.NewGenericEventPredicate(func(object client.Object) bool {
		du := object.(*velerov2alpha1api.DataUpload)
		return (du.Status.Phase == velerov2alpha1api.DataUploadPhaseAccepted)
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov2alpha1api.DataUpload{}).
		WatchesRawSource(s, nil, builder.WithPredicates(gp)).
		Watches(&corev1.Pod{}, kube.EnqueueRequestsFromMapUpdateFunc(r.findDataUploadForPod),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(ue event.UpdateEvent) bool {
					newObj := ue.ObjectNew.(*corev1.Pod)

					if _, ok := newObj.Labels[velerov1api.DataUploadLabel]; !ok {
						return false
					}

					if newObj.Spec.NodeName != r.nodeName {
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

func (r *DataUploadReconciler) findDataUploadForPod(ctx context.Context, podObj client.Object) []reconcile.Request {
	pod := podObj.(*corev1.Pod)
	du, err := findDataUploadByPod(r.client, *pod)
	log := r.logger.WithFields(logrus.Fields{
		"Backup pod": pod.Name,
	})

	if err != nil {
		log.WithError(err).Error("unable to get dataupload")
		return []reconcile.Request{}
	} else if du == nil {
		log.Error("get empty DataUpload")
		return []reconcile.Request{}
	}
	log = log.WithFields(logrus.Fields{
		"Datadupload": du.Name,
	})

	if du.Status.Phase != velerov2alpha1api.DataUploadPhaseAccepted {
		return []reconcile.Request{}
	}

	if pod.Status.Phase == corev1.PodRunning {
		log.Info("Preparing dataupload")
		// we don't expect anyone else update the CR during the Prepare process
		updated, err := r.exclusiveUpdateDataUpload(context.Background(), du, r.prepareDataUpload)
		if err != nil || !updated {
			log.WithField("updated", updated).WithError(err).Warn("failed to update dataupload, prepare will halt for this dataupload")
			return []reconcile.Request{}
		}
	} else if unrecoverable, reason := kube.IsPodUnrecoverable(pod, log); unrecoverable { // let the abnormal backup pod failed early
		err := UpdateDataUploadWithRetry(context.Background(), r.client, types.NamespacedName{Namespace: du.Namespace, Name: du.Name}, r.logger.WithField("dataupload", du.Name),
			func(dataUpload *velerov2alpha1api.DataUpload) bool {
				if dataUpload.Spec.Cancel {
					return false
				}

				dataUpload.Spec.Cancel = true
				dataUpload.Status.Message = fmt.Sprintf("Cancel dataupload because the exposing pod %s/%s is in abnormal status for reason %s", pod.Namespace, pod.Name, reason)

				return true
			})

		if err != nil {
			log.WithError(err).Warn("failed to cancel dataupload, and it will wait for prepare timeout")
			return []reconcile.Request{}
		}
		log.Infof("Exposed pod is in abnormal status(reason %s) and dataupload is marked as cancel", reason)
	} else {
		return []reconcile.Request{}
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: du.Namespace,
			Name:      du.Name,
		},
	}
	return []reconcile.Request{request}
}

func (r *DataUploadReconciler) prepareDataUpload(du *velerov2alpha1api.DataUpload) {
	du.Status.Phase = velerov2alpha1api.DataUploadPhasePrepared
	du.Status.Node = r.nodeName
}

func (r *DataUploadReconciler) errorOut(ctx context.Context, du *velerov2alpha1api.DataUpload, err error, msg string, log logrus.FieldLogger) (ctrl.Result, error) {
	if se, ok := r.snapshotExposerList[du.Spec.SnapshotType]; ok {
		var volumeSnapshotName string
		if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI { // Other exposer should have another condition
			volumeSnapshotName = du.Spec.CSISnapshot.VolumeSnapshot
		}
		se.CleanUp(ctx, getOwnerObject(du), volumeSnapshotName, du.Spec.SourceNamespace)
	} else {
		log.Errorf("failed to clean up exposed snapshot could not find %s snapshot exposer", du.Spec.SnapshotType)
	}

	return ctrl.Result{}, r.updateStatusToFailed(ctx, du, err, msg, log)
}

func (r *DataUploadReconciler) updateStatusToFailed(ctx context.Context, du *velerov2alpha1api.DataUpload, err error, msg string, log logrus.FieldLogger) error {
	original := du.DeepCopy()
	du.Status.Phase = velerov2alpha1api.DataUploadPhaseFailed
	du.Status.Message = errors.WithMessage(err, msg).Error()
	if du.Status.StartTimestamp.IsZero() {
		du.Status.StartTimestamp = &metav1.Time{Time: r.Clock.Now()}
	}

	if dataPathError, ok := err.(datapath.DataPathError); ok {
		du.Status.SnapshotID = dataPathError.GetSnapshotID()
	}
	du.Status.CompletionTimestamp = &metav1.Time{Time: r.Clock.Now()}
	if patchErr := r.client.Patch(ctx, du, client.MergeFrom(original)); patchErr != nil {
		log.WithError(patchErr).Error("error updating DataUpload status")
	} else {
		r.metrics.RegisterDataUploadFailure(r.nodeName)
	}

	return err
}

func (r *DataUploadReconciler) acceptDataUpload(ctx context.Context, du *velerov2alpha1api.DataUpload) (bool, error) {
	r.logger.Infof("Accepting data upload %s", du.Name)

	// For all data upload controller in each node-agent will try to update dataupload CR, and only one controller will success,
	// and the success one could handle later logic
	updated := du.DeepCopy()

	updateFunc := func(dataUpload *velerov2alpha1api.DataUpload) {
		dataUpload.Status.Phase = velerov2alpha1api.DataUploadPhaseAccepted
		labels := dataUpload.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[acceptNodeLabelKey] = r.nodeName
		dataUpload.SetLabels(labels)
	}

	succeeded, err := r.exclusiveUpdateDataUpload(ctx, updated, updateFunc)

	if err != nil {
		return false, err
	}

	if succeeded {
		updateFunc(du) // If update success, it's need to update du values in memory
		r.logger.WithField("Dataupload", du.Name).Infof("This datauplod has been accepted by %s", r.nodeName)
		return true, nil
	}

	r.logger.WithField("Dataupload", du.Name).Info("This datauplod has been accepted by others")
	return false, nil
}

func (r *DataUploadReconciler) onPrepareTimeout(ctx context.Context, du *velerov2alpha1api.DataUpload) {
	log := r.logger.WithField("Dataupload", du.Name)

	log.Info("Timeout happened for preparing dataupload")

	succeeded, err := r.exclusiveUpdateDataUpload(ctx, du, func(du *velerov2alpha1api.DataUpload) {
		du.Status.Phase = velerov2alpha1api.DataUploadPhaseFailed
		du.Status.Message = "timeout on preparing data upload"
	})

	if err != nil {
		log.WithError(err).Warn("Failed to update dataupload")
		return
	}

	if !succeeded {
		log.Warn("Dataupload has been updated by others")
		return
	}

	ep, ok := r.snapshotExposerList[du.Spec.SnapshotType]
	if !ok {
		log.WithError(fmt.Errorf("%v type of snapshot exposer is not exist", du.Spec.SnapshotType)).
			Warn("Failed to clean up resources on canceled")
	} else {
		var volumeSnapshotName string
		if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI { // Other exposer should have another condition
			volumeSnapshotName = du.Spec.CSISnapshot.VolumeSnapshot
		}

		ep.CleanUp(ctx, getOwnerObject(du), volumeSnapshotName, du.Spec.SourceNamespace)

		log.Info("Dataupload has been cleaned up")
	}

	r.metrics.RegisterDataUploadFailure(r.nodeName)
}

func (r *DataUploadReconciler) exclusiveUpdateDataUpload(ctx context.Context, du *velerov2alpha1api.DataUpload,
	updateFunc func(*velerov2alpha1api.DataUpload)) (bool, error) {
	updateFunc(du)

	err := r.client.Update(ctx, du)
	if err == nil {
		return true, nil
	}

	// warn we won't rollback du values in memory when error
	if apierrors.IsConflict(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (r *DataUploadReconciler) closeDataPath(ctx context.Context, duName string) {
	asyncBR := r.dataPathMgr.GetAsyncBR(duName)
	if asyncBR != nil {
		asyncBR.Close(ctx)
	}

	r.dataPathMgr.RemoveAsyncBR(duName)
}

func (r *DataUploadReconciler) setupExposeParam(du *velerov2alpha1api.DataUpload) (interface{}, error) {
	if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI {
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.client.Get(context.Background(), types.NamespacedName{
			Namespace: du.Spec.SourceNamespace,
			Name:      du.Spec.SourcePVC,
		}, pvc)

		if err != nil {
			return nil, errors.Wrapf(err, "failed to get PVC %s/%s", du.Spec.SourceNamespace, du.Spec.SourcePVC)
		}

		accessMode := exposer.AccessModeFileSystem
		if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
			accessMode = exposer.AccessModeBlock
		}

		return &exposer.CSISnapshotExposeParam{
			SnapshotName:     du.Spec.CSISnapshot.VolumeSnapshot,
			SourceNamespace:  du.Spec.SourceNamespace,
			StorageClass:     du.Spec.CSISnapshot.StorageClass,
			HostingPodLabels: map[string]string{velerov1api.DataUploadLabel: du.Name},
			AccessMode:       accessMode,
			OperationTimeout: du.Spec.OperationTimeout.Duration,
			ExposeTimeout:    r.preparingTimeout,
			VolumeSize:       pvc.Spec.Resources.Requests[corev1.ResourceStorage],
			Affinity:         r.loadAffinity,
			BackupPVCConfig:  r.backupPVCConfig,
			Resources:        r.podResources,
		}, nil
	}
	return nil, nil
}

func (r *DataUploadReconciler) setupWaitExposePara(du *velerov2alpha1api.DataUpload) interface{} {
	if du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI {
		return &exposer.CSISnapshotExposeWaitParam{
			NodeClient: r.client,
			NodeName:   r.nodeName,
		}
	}
	return nil
}

func getOwnerObject(du *velerov2alpha1api.DataUpload) corev1.ObjectReference {
	return corev1.ObjectReference{
		Kind:       du.Kind,
		Namespace:  du.Namespace,
		Name:       du.Name,
		UID:        du.UID,
		APIVersion: du.APIVersion,
	}
}

func findDataUploadByPod(client client.Client, pod corev1.Pod) (*velerov2alpha1api.DataUpload, error) {
	if label, exist := pod.Labels[velerov1api.DataUploadLabel]; exist {
		du := &velerov2alpha1api.DataUpload{}
		err := client.Get(context.Background(), types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      label,
		}, du)

		if err != nil {
			return nil, errors.Wrapf(err, "error to find DataUpload by pod %s/%s", pod.Namespace, pod.Name)
		}
		return du, nil
	}
	return nil, nil
}

func isDataUploadInFinalState(du *velerov2alpha1api.DataUpload) bool {
	return du.Status.Phase == velerov2alpha1api.DataUploadPhaseFailed ||
		du.Status.Phase == velerov2alpha1api.DataUploadPhaseCanceled ||
		du.Status.Phase == velerov2alpha1api.DataUploadPhaseCompleted
}

func UpdateDataUploadWithRetry(ctx context.Context, client client.Client, namespacedName types.NamespacedName, log *logrus.Entry, updateFunc func(*velerov2alpha1api.DataUpload) bool) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		du := &velerov2alpha1api.DataUpload{}
		if err := client.Get(ctx, namespacedName, du); err != nil {
			return false, errors.Wrap(err, "getting DataUpload")
		}

		if updateFunc(du) {
			err := client.Update(ctx, du)
			if err != nil {
				if apierrors.IsConflict(err) {
					log.Warnf("failed to update dataupload for %s/%s and will retry it", du.Namespace, du.Name)
					return false, nil
				} else {
					return false, errors.Wrapf(err, "error updating dataupload with error %s/%s", du.Namespace, du.Name)
				}
			}
		}

		return true, nil
	})
}

var funcResumeCancellableDataBackup = (*DataUploadReconciler).resumeCancellableDataPath

func (r *DataUploadReconciler) AttemptDataUploadResume(ctx context.Context, logger *logrus.Entry, ns string) error {
	dataUploads := &velerov2alpha1api.DataUploadList{}
	if err := r.client.List(ctx, dataUploads, &client.ListOptions{Namespace: ns}); err != nil {
		r.logger.WithError(errors.WithStack(err)).Error("failed to list datauploads")
		return errors.Wrapf(err, "error to list datauploads")
	}

	for i := range dataUploads.Items {
		du := &dataUploads.Items[i]
		if du.Status.Phase == velerov2alpha1api.DataUploadPhasePrepared {
			// keep doing nothing let controller re-download the data
			// the Prepared CR could be still handled by dataupload controller after node-agent restart
			logger.WithField("dataupload", du.GetName()).Debug("find a dataupload with status prepared")
		} else if du.Status.Phase == velerov2alpha1api.DataUploadPhaseInProgress {
			if du.Status.Node != r.nodeName {
				logger.WithField("du", du.Name).WithField("current node", r.nodeName).Infof("DU should be resumed by another node %s", du.Status.Node)
				continue
			}

			err := funcResumeCancellableDataBackup(r, ctx, du, logger)
			if err == nil {
				logger.WithField("du", du.Name).WithField("current node", r.nodeName).Info("Completed to resume in progress DU")
				continue
			}

			logger.WithField("dataupload", du.GetName()).WithError(err).Warn("Failed to resume data path for du, have to cancel it")

			resumeErr := err
			err = UpdateDataUploadWithRetry(ctx, r.client, types.NamespacedName{Namespace: du.Namespace, Name: du.Name}, logger.WithField("dataupload", du.Name),
				func(dataUpload *velerov2alpha1api.DataUpload) bool {
					if dataUpload.Spec.Cancel {
						return false
					}

					dataUpload.Spec.Cancel = true
					dataUpload.Status.Message = fmt.Sprintf("Resume InProgress dataupload failed with error %v, mark it as cancel", resumeErr)

					return true
				})
			if err != nil {
				logger.WithField("dataupload", du.GetName()).WithError(errors.WithStack(err)).Error("Failed to trigger dataupload cancel")
			}
		} else if du.Status.Phase == velerov2alpha1api.DataUploadPhaseAccepted {
			r.logger.WithField("dataupload", du.GetName()).Warn("Cancel du under Accepted phase")

			err := UpdateDataUploadWithRetry(ctx, r.client, types.NamespacedName{Namespace: du.Namespace, Name: du.Name}, r.logger.WithField("dataupload", du.Name),
				func(dataUpload *velerov2alpha1api.DataUpload) bool {
					if dataUpload.Spec.Cancel {
						return false
					}

					dataUpload.Spec.Cancel = true
					dataUpload.Status.Message = "Dataupload is in Accepted status during the node-agent starting, mark it as cancel"

					return true
				})
			if err != nil {
				r.logger.WithField("dataupload", du.GetName()).WithError(errors.WithStack(err)).Error("Failed to trigger dataupload cancel")
			}
		}
	}

	return nil
}

func (r *DataUploadReconciler) resumeCancellableDataPath(ctx context.Context, du *velerov2alpha1api.DataUpload, log logrus.FieldLogger) error {
	log.Info("Resume cancelable dataUpload")

	ep, ok := r.snapshotExposerList[du.Spec.SnapshotType]
	if !ok {
		return errors.Errorf("error to find exposer for du %s", du.Name)
	}

	waitExposePara := r.setupWaitExposePara(du)
	res, err := ep.GetExposed(ctx, getOwnerObject(du), du.Spec.OperationTimeout.Duration, waitExposePara)
	if err != nil {
		return errors.Wrapf(err, "error to get exposed snapshot for du %s", du.Name)
	}

	if res == nil {
		return errors.Errorf("expose info missed for du %s", du.Name)
	}

	callbacks := datapath.Callbacks{
		OnCompleted: r.OnDataUploadCompleted,
		OnFailed:    r.OnDataUploadFailed,
		OnCancelled: r.OnDataUploadCancelled,
		OnProgress:  r.OnDataUploadProgress,
	}

	asyncBR, err := r.dataPathMgr.CreateMicroServiceBRWatcher(ctx, r.client, r.kubeClient, r.mgr, datapath.TaskTypeBackup, du.Name, du.Namespace, res.ByPod.HostingPod.Name, res.ByPod.HostingContainer, du.Name, callbacks, true, log)
	if err != nil {
		return errors.Wrapf(err, "error to create asyncBR watcher for du %s", du.Name)
	}

	resumeComplete := false
	defer func() {
		if !resumeComplete {
			r.closeDataPath(ctx, du.Name)
		}
	}()

	if err := asyncBR.Init(ctx, nil); err != nil {
		return errors.Wrapf(err, "error to init asyncBR watcher for du %s", du.Name)
	}

	if err := asyncBR.StartBackup(datapath.AccessPoint{
		ByPath: res.ByPod.VolumeName,
	}, du.Spec.DataMoverConfig, nil); err != nil {
		return errors.Wrapf(err, "error to resume asyncBR watcher for du %s", du.Name)
	}

	resumeComplete = true

	log.Infof("asyncBR is resumed for du %s", du.Name)

	return nil
}
