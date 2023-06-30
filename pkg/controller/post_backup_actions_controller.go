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
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type postBackupActionsReconciler struct {
	logger            logrus.FieldLogger
	logLevel          logrus.Level
	logFormat         logging.Format
	client            kbclient.Client
	metrics           *metrics.ServerMetrics
	backupStoreGetter persistence.ObjectBackupStoreGetter
	newPluginManager  func(logrus.FieldLogger) clientmgmt.Manager
}

func NewPostBackupActionsReconciler(
	logger logrus.FieldLogger,
	logLevel logrus.Level,
	logFormat logging.Format,
	client kbclient.Client,
	metrics *metrics.ServerMetrics,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
) *postBackupActionsReconciler {
	return &postBackupActionsReconciler{
		logger:            logger,
		logLevel:          logLevel,
		logFormat:         logFormat,
		client:            client,
		metrics:           metrics,
		backupStoreGetter: backupStoreGetter,
		newPluginManager:  newPluginManager,
	}
}

func (b *postBackupActionsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Backup{}).
		Complete(b)
}

func (b *postBackupActionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := b.logger.WithFields(logrus.Fields{
		"controller": "post-backup-actions-controller",
		"backup":     req.NamespacedName,
	})

	log.Info("Getting Backup")
	backup := &api.Backup{}
	if err := b.client.Get(ctx, req.NamespacedName, backup); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find Backup")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting Backup")
		return ctrl.Result{}, errors.WithStack(err)
	}

	switch backup.Status.Phase {
	case api.BackupPhaseWaitingForPostBackupActions:
		b.logger.Info("Backup wait post actions, proceeded")
	default:
		return ctrl.Result{}, nil
	}

	original := backup.DeepCopy()

	log.Info("Getting storage location")
	storageLocation := &api.BackupStorageLocation{}
	if err := b.client.Get(context.Background(), kbclient.ObjectKey{
		Namespace: req.Namespace,
		Name:      backup.Spec.StorageLocation,
	}, storageLocation); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find StorageLocation")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting StorageLocation")
		return ctrl.Result{}, errors.WithStack(err)
	}

	err := b.runPostBackupActions(backup, storageLocation)
	if err != nil {
		log.WithError(err).Error("Post backup actions failed")
		backup.Status.Phase = api.BackupPhaseFailedPostBackupActions
		backup.Status.FailureReason = err.Error()
	} else {
		log.Info("Post backup actions succeeded")
		backup.Status.Phase = api.BackupPhaseCompleted
		b.metrics.RegisterBackupSuccess(backup.Name)
		b.metrics.RegisterBackupLastStatus(backup.Name, metrics.BackupLastStatusSucc)
	}
	backup.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}

	log.Debug("Updating backup's final status")
	if err = kubeutil.PatchResource(original, backup, b.client); err != nil {
		log.WithError(errors.WithStack(err)).
			Infof("Error updating backup's final status, post restore actions completed with status: %s", backup.Status.Phase)
		// TODO maybe add retry or tracker of not updated backups
	}
	return ctrl.Result{}, nil
}

func (b *postBackupActionsReconciler) runPostBackupActions(backup *api.Backup, storageLocation *api.BackupStorageLocation) error {
	b.logger.Info("Setting up post backup logger")
	actionsLogCounter := logging.NewLogHook()
	actionsLog, err := logging.NewTempFileLogger(b.logLevel, b.logFormat, actionsLogCounter, logrus.Fields{Backup: kubeutil.NamespaceAndName(backup)})
	if err != nil {
		return errors.Wrap(err, "error creating dual mode logger for post backup actions")
	}
	defer actionsLog.Dispose(b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)))

	b.logger.Info("Setting up plugin manager")
	pluginManager := b.newPluginManager(actionsLog)
	defer pluginManager.CleanupClients()

	actionsLog.Info("Getting post backup actions")
	actions, err := pluginManager.GetPostBackupActions()
	if err != nil {
		return errors.Wrap(err, "error getting post backup actions")
	}

	var postBackupErr error
	if len(actions) > 0 {
		actionsLog.Infof("Post backup actions are present, running them")
		for _, postBackupAction := range actions {
			postBackupErr = postBackupAction.Execute(backup)
			if postBackupErr != nil {
				actionsLog.Errorf("Error running post backup action, error: %s", postBackupErr)
				backup.Status.Phase = api.BackupPhaseFailedPostBackupActions
				break
			}
		}
	} else {
		actionsLog.Info("Post backup action are not present")
	}

	actionsLog.Info("Setting up backup store to persist post backup log")
	backupStore, err := b.backupStoreGetter.Get(storageLocation, pluginManager, actionsLog)
	if err != nil {
		return err
	}
	actionsLog.Info("Persisting post backup logs")
	actionsLog.DoneForPersist(b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)))
	actionsLogFile, err := actionsLog.GetPersistFile()
	if err != nil {
		actionsLog.Errorf("Error while persisting post backup log, err: %s", err.Error())
		return err
	}
	backupStore.PutPostBackupLog(backup.Name, actionsLogFile)
	// update backup metadata in object store
	backupJSON := new(bytes.Buffer)
	if err := encode.To(backup, "json", backupJSON); err != nil {
		return errors.Wrap(err, "error encoding backup json")
	}
	err = backupStore.PutBackupMetadata(backup.Name, backupJSON)
	if err != nil {
		return errors.Wrap(err, "error persisting backup metadata")
	}
	return postBackupErr
}
