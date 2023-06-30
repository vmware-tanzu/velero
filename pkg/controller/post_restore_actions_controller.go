package controller

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type postRestoreActionsReconciler struct {
	ctx                         context.Context
	namespace                   string
	client                      client.Client
	restoreLogLevel             logrus.Level
	logger                      logrus.FieldLogger
	metrics                     *metrics.ServerMetrics
	logFormat                   logging.Format
	clock                       clock.WithTickerAndDelayedExecution
	defaultItemOperationTimeout time.Duration

	newPluginManager  func(logger logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter
}

func NewPostRestoreActionsReconciler(
	ctx context.Context,
	namespace string,
	client client.Client,
	logger logrus.FieldLogger,
	restoreLogLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	metrics *metrics.ServerMetrics,
	logFormat logging.Format,
	defaultItemOperationTimeout time.Duration,
) *postRestoreActionsReconciler {
	r := &postRestoreActionsReconciler{
		ctx:                         ctx,
		namespace:                   namespace,
		client:                      client,
		logger:                      logger,
		restoreLogLevel:             restoreLogLevel,
		metrics:                     metrics,
		logFormat:                   logFormat,
		clock:                       &clock.RealClock{},
		defaultItemOperationTimeout: defaultItemOperationTimeout,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
	}

	return r
}

func (r *postRestoreActionsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Restore{}).
		Complete(r)
}

func (r *postRestoreActionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Developer note: any error returned by this method will
	// cause the restore to be re-enqueued and re-processed by
	// the controller.
	log := r.logger.WithFields(logrus.Fields{
		"controller": "post-restore-actions-controller",
		"restore":    req.NamespacedName,
	})

	restore := &api.Restore{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, restore)
	if err != nil {
		log.Infof("Fail to get restore %s: %s", req.NamespacedName.String(), err.Error())
		return ctrl.Result{}, err
	}

	switch restore.Status.Phase {
	case api.RestorePhaseWaitingForPostRestoreActions:
		log.Info("Restore waiting post actions, proceed")
	default:
		return ctrl.Result{}, nil
	}

	// store a copy of the original restore for creating patch
	original := restore.DeepCopy()

	backupScheduleName := restore.Spec.ScheduleName

	info, err := fetchBackupInfoInternal(r.client, r.namespace, restore.Spec.BackupName)
	if err != nil {
		r.logger.WithError(err).Error("Error fetching backup info")
		return ctrl.Result{}, err
	}

	if err := r.runPostRestoreActions(restore, info); err != nil {
		log.WithError(err).Debug("Post actions failed")
		restore.Status.Phase = api.RestorePhaseFailedPostRestoreActions
		restore.Status.FailureReason = err.Error()
	} else {
		log.Info("Post actions completed")
		restore.Status.Phase = api.RestorePhaseCompleted
		r.metrics.RegisterBackupSuccess(backupScheduleName)
	}
	restore.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}

	log.Debug("Updating restore's final status")
	if err = kubeutil.PatchResource(original, restore, r.client); err != nil {
		log.WithError(errors.WithStack(err)).
			Infof("Error updating restore's final status, post restore actions completed with status: %s", restore.Status.Phase)
		// TODO add retry or tracker of not updated restores
	}

	return ctrl.Result{}, nil
}

func (r *postRestoreActionsReconciler) runPostRestoreActions(restore *api.Restore, info backupInfo) error {
	postRestoreLog, err := logging.NewTempFileLogger(r.restoreLogLevel, r.logFormat, nil, logrus.Fields{"restore": kubeutil.NamespaceAndName(restore)})
	if err != nil {
		return err
	}
	defer postRestoreLog.Dispose(r.logger)

	postRestorePluginManager := r.newPluginManager(postRestoreLog)
	defer postRestorePluginManager.CleanupClients()

	postRestoreLog.Info("Getting post restore actions")
	postRestoreActions, err := postRestorePluginManager.GetPostRestoreActions()
	if err != nil {
		return errors.Wrap(err, "error getting post restore actions")
	}

	var postRestoreError error = nil
	if len(postRestoreActions) > 0 {
		postRestoreLog.Info("Post restore actions are present, running them")
		for _, action := range postRestoreActions {
			err := action.Execute(restore)
			if err != nil {
				postRestoreLog.Errorf("Error executing post restore action: %s", err)
				restore.Status.Phase = api.RestorePhaseFailedPostRestoreActions
				postRestoreError = errors.Wrap(err, "error executing post restore action")
				break
			}
		}
	} else {
		postRestoreLog.Info("Post restore actions are not present")
	}

	pluginManager := r.newPluginManager(postRestoreLog)
	defer pluginManager.CleanupClients()
	backupStore, err := r.backupStoreGetter.Get(info.location, pluginManager, r.logger)
	if err != nil {
		return err
	}

	postRestoreLog.Info("Persisting post restore log")
	postRestoreLog.DoneForPersist(r.logger.WithField(Restore, restore.Name))
	postRestoreLogFile, err := postRestoreLog.GetPersistFile()
	if err != nil {
		r.logger.WithError(err).WithField("restore", restore.Name).Error("Error getting post restore log file")
	} else {
		backupStore.PutPostRestoreLog(restore.Spec.BackupName, restore.Name, postRestoreLogFile)
	}
	return postRestoreError
}
