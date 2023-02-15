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
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/hook"
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	pkgrestore "github.com/vmware-tanzu/velero/pkg/restore"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

// nonRestorableResources is an exclusion list  for the restoration process. Any resources
// included here are explicitly excluded from the restoration process.
var nonRestorableResources = []string{
	"nodes",
	"events",
	"events.events.k8s.io",

	// Don't ever restore backups - if appropriate, they'll be synced in from object storage.
	// https://github.com/vmware-tanzu/velero/issues/622
	"backups.velero.io",

	// Restores are cluster-specific, and don't have value moving across clusters.
	// https://github.com/vmware-tanzu/velero/issues/622
	"restores.velero.io",

	// TODO: Remove this in v1.11 or v1.12
	// Restic repositories are automatically managed by Velero and will be automatically
	// created as needed if they don't exist.
	// https://github.com/vmware-tanzu/velero/issues/1113
	"resticrepositories.velero.io",

	// CSINode delegates cluster node for CSI operation.
	// VolumeAttachement records PV mounts to which node.
	// https://github.com/vmware-tanzu/velero/issues/4823
	"csinodes.storage.k8s.io",
	"volumeattachments.storage.k8s.io",

	// Backup repositories were renamed from Restic repositories
	"backuprepositories.velero.io",
}

type restoreReconciler struct {
	ctx             context.Context
	namespace       string
	restorer        pkgrestore.Restorer
	kbClient        client.Client
	restoreLogLevel logrus.Level
	logger          logrus.FieldLogger
	metrics         *metrics.ServerMetrics
	logFormat       logging.Format
	clock           clock.WithTickerAndDelayedExecution

	newPluginManager  func(logger logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter
}

type backupInfo struct {
	backup      *api.Backup
	location    *api.BackupStorageLocation
	backupStore persistence.BackupStore
}

func NewRestoreReconciler(
	ctx context.Context,
	namespace string,
	restorer pkgrestore.Restorer,
	kbClient client.Client,
	logger logrus.FieldLogger,
	restoreLogLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	metrics *metrics.ServerMetrics,
	logFormat logging.Format,
) *restoreReconciler {
	r := &restoreReconciler{
		ctx:             ctx,
		namespace:       namespace,
		restorer:        restorer,
		kbClient:        kbClient,
		logger:          logger,
		restoreLogLevel: restoreLogLevel,
		metrics:         metrics,
		logFormat:       logFormat,
		clock:           &clock.RealClock{},

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
	}

	// Move the periodical backup and restore metrics computing logic from controllers to here.
	// This is due to, after controllers using controller-runtime, controllers doesn't have a
	// timer as the before generic-controller, and the backup and restore controller only have
	// one length queue, furthermore the backup and restore process could last for a long time.
	// Compute the metric here is a better choice.
	r.updateTotalRestoreMetric()

	return r
}

func (r *restoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Developer note: any error returned by this method will
	// cause the restore to be re-enqueued and re-processed by
	// the controller.
	log := r.logger.WithField("Restore", req.NamespacedName.String())

	restore := &api.Restore{}
	err := r.kbClient.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, restore)
	if err != nil {
		log.Infof("Fail to get restore %s: %s", req.NamespacedName.String(), err.Error())
		return ctrl.Result{}, err
	}

	// store a copy of the original restore for creating patch
	original := restore.DeepCopy()

	// Validate the restore and fetch the backup. Note that the plugin
	// manager used here is not the same one used by c.runValidatedRestore,
	// since within that function we want the plugin manager to log to
	// our per-restore log (which is instantiated within c.runValidatedRestore).
	pluginManager := r.newPluginManager(r.logger)
	defer pluginManager.CleanupClients()
	info := r.validateAndComplete(restore, pluginManager)

	// Register attempts after validation so we don't have to fetch the backup multiple times
	backupScheduleName := restore.Spec.ScheduleName
	r.metrics.RegisterRestoreAttempt(backupScheduleName)

	if len(restore.Status.ValidationErrors) > 0 {
		restore.Status.Phase = api.RestorePhaseFailedValidation
		r.metrics.RegisterRestoreValidationFailed(backupScheduleName)
	} else {
		restore.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now()}
		restore.Status.Phase = api.RestorePhaseInProgress
	}

	// patch to update status and persist to API
	err = kubeutil.PatchResource(original, restore, r.kbClient)
	if err != nil {
		// return the error so the restore can be re-processed; it's currently
		// still in phase = New.
		log.Errorf("fail to update restore %s status to %s: %s",
			req.NamespacedName.String(), restore.Status.Phase, err.Error())
		return ctrl.Result{}, errors.Wrapf(err, "error updating Restore phase to %s", restore.Status.Phase)
	}
	// store ref to just-updated item for creating patch
	original = restore.DeepCopy()

	if restore.Status.Phase == api.RestorePhaseFailedValidation {
		return ctrl.Result{}, nil
	}

	if err := r.runValidatedRestore(restore, info); err != nil {
		log.WithError(err).Debug("Restore failed")
		restore.Status.Phase = api.RestorePhaseFailed
		restore.Status.FailureReason = err.Error()
		r.metrics.RegisterRestoreFailed(backupScheduleName)
	} else if restore.Status.Errors > 0 {
		log.Debug("Restore partially failed")
		restore.Status.Phase = api.RestorePhasePartiallyFailed
		r.metrics.RegisterRestorePartialFailure(backupScheduleName)
	} else {
		log.Debug("Restore completed")
		restore.Status.Phase = api.RestorePhaseCompleted
		r.metrics.RegisterRestoreSuccess(backupScheduleName)
	}

	restore.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
	log.Debug("Updating restore's final status")
	if err = kubeutil.PatchResource(original, restore, r.kbClient); err != nil {
		log.WithError(errors.WithStack(err)).Info("Error updating restore's final status")
		// No need to re-enqueue here, because restore's already set to InProgress before.
		// Controller only handle New restore.
	}

	return ctrl.Result{}, nil
}

func (r *restoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(kubeutil.NewCreateEventPredicate(func(obj client.Object) bool {
			restore := obj.(*api.Restore)

			switch restore.Status.Phase {
			case "", api.RestorePhaseNew:
				// only process new restores
				return true
			default:
				r.logger.WithFields(logrus.Fields{
					"restore": kubeutil.NamespaceAndName(restore),
					"phase":   restore.Status.Phase,
				}).Debug("Restore is not new, skipping")
				return false
			}
		})).
		For(&api.Restore{}).
		Complete(r)
}

func (r *restoreReconciler) validateAndComplete(restore *api.Restore, pluginManager clientmgmt.Manager) backupInfo {
	// add non-restorable resources to restore's excluded resources
	excludedResources := sets.NewString(restore.Spec.ExcludedResources...)
	for _, nonrestorable := range nonRestorableResources {
		if !excludedResources.Has(nonrestorable) {
			restore.Spec.ExcludedResources = append(restore.Spec.ExcludedResources, nonrestorable)
		}
	}

	// validate that included resources don't contain any non-restorable resources
	includedResources := sets.NewString(restore.Spec.IncludedResources...)
	for _, nonRestorableResource := range nonRestorableResources {
		if includedResources.Has(nonRestorableResource) {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("%v are non-restorable resources", nonRestorableResource))
		}
	}

	// validate included/excluded resources
	for _, err := range collections.ValidateIncludesExcludes(restore.Spec.IncludedResources, restore.Spec.ExcludedResources) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	// validate included/excluded namespaces
	for _, err := range collections.ValidateIncludesExcludes(restore.Spec.IncludedNamespaces, restore.Spec.ExcludedNamespaces) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	// validate that only one exists orLabelSelector or just labelSelector (singular)
	if restore.Spec.OrLabelSelectors != nil && restore.Spec.LabelSelector != nil {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("encountered labelSelector as well as orLabelSelectors in restore spec, only one can be specified"))
	}

	// validate that exactly one of BackupName and ScheduleName have been specified
	if !backupXorScheduleProvided(restore) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Either a backup or schedule must be specified as a source for the restore, but not both")
		return backupInfo{}
	}

	// validate Restore Init Hook's InitContainers
	restoreHooks, err := hook.GetRestoreHooksFromSpec(&restore.Spec.Hooks)
	if err != nil {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, err.Error())
	}
	for _, resource := range restoreHooks {
		for _, h := range resource.RestoreHooks {
			if h.Init != nil {
				for _, container := range h.Init.InitContainers {
					err = hook.ValidateContainer(container.Raw)
					if err != nil {
						restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, err.Error())
					}
				}
			}
		}
	}

	// if ScheduleName is specified, fill in BackupName with the most recent successful backup from
	// the schedule
	if restore.Spec.ScheduleName != "" {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{
			api.ScheduleNameLabel: restore.Spec.ScheduleName,
		}))

		backupList := &api.BackupList{}
		r.kbClient.List(context.Background(), backupList, &client.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Unable to list backups for schedule")
			return backupInfo{}
		}
		if len(backupList.Items) == 0 {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "No backups found for schedule")
		}

		if backup := mostRecentCompletedBackup(backupList.Items); backup.Name != "" {
			restore.Spec.BackupName = backup.Name
		} else {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "No completed backups found for schedule")
			return backupInfo{}
		}
	}

	info, err := r.fetchBackupInfo(restore.Spec.BackupName, pluginManager)
	if err != nil {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Error retrieving backup: %v", err))
		return backupInfo{}
	}

	// Fill in the ScheduleName so it's easier to consume for metrics.
	if restore.Spec.ScheduleName == "" {
		restore.Spec.ScheduleName = info.backup.GetLabels()[api.ScheduleNameLabel]
	}

	return info
}

// backupXorScheduleProvided returns true if exactly one of BackupName and
// ScheduleName are non-empty for the restore, or false otherwise.
func backupXorScheduleProvided(restore *api.Restore) bool {
	if restore.Spec.BackupName != "" && restore.Spec.ScheduleName != "" {
		return false
	}

	if restore.Spec.BackupName == "" && restore.Spec.ScheduleName == "" {
		return false
	}

	return true
}

// mostRecentCompletedBackup returns the most recent backup that's
// completed from a list of backups.
func mostRecentCompletedBackup(backups []api.Backup) api.Backup {
	sort.Slice(backups, func(i, j int) bool {
		// Use .After() because we want descending sort.

		var iStartTime, jStartTime time.Time
		if backups[i].Status.StartTimestamp != nil {
			iStartTime = backups[i].Status.StartTimestamp.Time
		}
		if backups[j].Status.StartTimestamp != nil {
			jStartTime = backups[j].Status.StartTimestamp.Time
		}
		return iStartTime.After(jStartTime)
	})

	for _, backup := range backups {
		if backup.Status.Phase == api.BackupPhaseCompleted {
			return backup
		}
	}

	return api.Backup{}
}

// fetchBackupInfo checks the backup lister for a backup that matches the given name. If it doesn't
// find it, it returns an error.
func (r *restoreReconciler) fetchBackupInfo(backupName string, pluginManager clientmgmt.Manager) (backupInfo, error) {
	backup := &api.Backup{}
	err := r.kbClient.Get(context.Background(), types.NamespacedName{Namespace: r.namespace, Name: backupName}, backup)
	if err != nil {
		return backupInfo{}, err
	}

	location := &api.BackupStorageLocation{}
	if err := r.kbClient.Get(context.Background(), client.ObjectKey{
		Namespace: r.namespace,
		Name:      backup.Spec.StorageLocation,
	}, location); err != nil {
		return backupInfo{}, errors.WithStack(err)
	}

	backupStore, err := r.backupStoreGetter.Get(location, pluginManager, r.logger)
	if err != nil {
		return backupInfo{}, err
	}

	return backupInfo{
		backup:      backup,
		location:    location,
		backupStore: backupStore,
	}, nil
}

// runValidatedRestore takes a validated restore API object and executes the restore process.
// The log and results files are uploaded to backup storage. Any error returned from this function
// means that the restore failed. This function updates the restore API object with warning and error
// counts, but *does not* update its phase or patch it via the API.
func (r *restoreReconciler) runValidatedRestore(restore *api.Restore, info backupInfo) error {
	// instantiate the per-restore logger that will output both to a temp file
	// (for upload to object storage) and to stdout.
	restoreLog, err := newRestoreLogger(restore, r.restoreLogLevel, r.logFormat)
	if err != nil {
		return err
	}
	defer restoreLog.closeAndRemove(r.logger)

	pluginManager := r.newPluginManager(restoreLog)
	defer pluginManager.CleanupClients()

	actions, err := pluginManager.GetRestoreItemActionsV2()
	if err != nil {
		return errors.Wrap(err, "error getting restore item actions")
	}
	actionsResolver := framework.NewRestoreItemActionResolverV2(actions)

	itemSnapshotters, err := pluginManager.GetItemSnapshotters()
	if err != nil {
		return errors.Wrap(err, "error getting item snapshotters")
	}
	snapshotItemResolver := framework.NewItemSnapshotterResolver(itemSnapshotters)

	backupFile, err := downloadToTempFile(restore.Spec.BackupName, info.backupStore, restoreLog)
	if err != nil {
		return errors.Wrap(err, "error downloading backup")
	}
	defer closeAndRemoveFile(backupFile, r.logger)

	listOpts := &client.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			api.BackupNameLabel: label.GetValidName(restore.Spec.BackupName),
		}).AsSelector(),
	}

	podVolumeBackupList := &api.PodVolumeBackupList{}
	err = r.kbClient.List(context.TODO(), podVolumeBackupList, listOpts)
	if err != nil {
		restoreLog.Errorf("Fail to list PodVolumeBackup :%s", err.Error())
		return errors.WithStack(err)
	}

	volumeSnapshots, err := info.backupStore.GetBackupVolumeSnapshots(restore.Spec.BackupName)
	if err != nil {
		return errors.Wrap(err, "error fetching volume snapshots metadata")
	}

	restoreLog.Info("starting restore")

	var podVolumeBackups []*api.PodVolumeBackup
	for i := range podVolumeBackupList.Items {
		podVolumeBackups = append(podVolumeBackups, &podVolumeBackupList.Items[i])
	}
	restoreReq := pkgrestore.Request{
		Log:              restoreLog,
		Restore:          restore,
		Backup:           info.backup,
		PodVolumeBackups: podVolumeBackups,
		VolumeSnapshots:  volumeSnapshots,
		BackupReader:     backupFile,
	}
	restoreWarnings, restoreErrors := r.restorer.RestoreWithResolvers(restoreReq, actionsResolver, snapshotItemResolver,
		pluginManager)

	// log errors and warnings to the restore log
	for _, msg := range restoreErrors.Velero {
		restoreLog.Errorf("Velero restore error: %v", msg)
	}
	for _, msg := range restoreErrors.Cluster {
		restoreLog.Errorf("Cluster resource restore error: %v", msg)
	}
	for ns, errs := range restoreErrors.Namespaces {
		for _, msg := range errs {
			restoreLog.Errorf("Namespace %v, resource restore error: %v", ns, msg)
		}
	}
	for _, msg := range restoreWarnings.Velero {
		restoreLog.Warnf("Velero restore warning: %v", msg)
	}
	for _, msg := range restoreWarnings.Cluster {
		restoreLog.Warnf("Cluster resource restore warning: %v", msg)
	}
	for ns, errs := range restoreWarnings.Namespaces {
		for _, msg := range errs {
			restoreLog.Warnf("Namespace %v, resource restore warning: %v", ns, msg)
		}
	}
	restoreLog.Info("restore completed")

	// re-instantiate the backup store because credentials could have changed since the original
	// instantiation, if this was a long-running restore
	info.backupStore, err = r.backupStoreGetter.Get(info.location, pluginManager, r.logger)
	if err != nil {
		return errors.Wrap(err, "error setting up backup store to persist log and results files")
	}

	if logReader, err := restoreLog.done(r.logger); err != nil {
		restoreErrors.Velero = append(restoreErrors.Velero, fmt.Sprintf("error getting restore log reader: %v", err))
	} else {
		if err := info.backupStore.PutRestoreLog(restore.Spec.BackupName, restore.Name, logReader); err != nil {
			restoreErrors.Velero = append(restoreErrors.Velero, fmt.Sprintf("error uploading log file to backup storage: %v", err))
		}
	}

	// At this point, no further logs should be written to restoreLog since it's been uploaded
	// to object storage.

	restore.Status.Warnings = len(restoreWarnings.Velero) + len(restoreWarnings.Cluster)
	for _, w := range restoreWarnings.Namespaces {
		restore.Status.Warnings += len(w)
	}

	restore.Status.Errors = len(restoreErrors.Velero) + len(restoreErrors.Cluster)
	for _, e := range restoreErrors.Namespaces {
		restore.Status.Errors += len(e)
	}

	m := map[string]results.Result{
		"warnings": restoreWarnings,
		"errors":   restoreErrors,
	}

	if err := putResults(restore, m, info.backupStore); err != nil {
		r.logger.WithError(err).Error("Error uploading restore results to backup storage")
	}

	return nil
}

// updateTotalRestoreMetric update the velero_restore_total metric every minute.
func (r *restoreReconciler) updateTotalRestoreMetric() {
	go func() {
		// Wait for 5 seconds to let controller-runtime to setup k8s clients.
		time.Sleep(5 * time.Second)

		wait.NonSlidingUntil(
			func() {
				// recompute restore_total metric
				restoreList := &api.RestoreList{}
				err := r.kbClient.List(context.Background(), restoreList, &client.ListOptions{})
				if err != nil {
					r.logger.Error(err, "Error computing restore_total metric")
				} else {
					r.metrics.SetRestoreTotal(int64(len(restoreList.Items)))
				}
			},
			1*time.Minute,
			r.ctx.Done(),
		)
	}()
}

func putResults(restore *api.Restore, results map[string]results.Result, backupStore persistence.BackupStore) error {
	buf := new(bytes.Buffer)
	gzw := gzip.NewWriter(buf)
	defer gzw.Close()

	if err := json.NewEncoder(gzw).Encode(results); err != nil {
		return errors.Wrap(err, "error encoding restore results to JSON")
	}

	if err := gzw.Close(); err != nil {
		return errors.Wrap(err, "error closing gzip writer")
	}

	if err := backupStore.PutRestoreResults(restore.Spec.BackupName, restore.Name, buf); err != nil {
		return err
	}

	return nil
}

func downloadToTempFile(backupName string, backupStore persistence.BackupStore, logger logrus.FieldLogger) (*os.File, error) {
	readCloser, err := backupStore.GetBackupContents(backupName)
	if err != nil {
		return nil, err
	}
	defer readCloser.Close()

	file, err := ioutil.TempFile("", backupName)
	if err != nil {
		return nil, errors.Wrap(err, "error creating Backup temp file")
	}

	n, err := io.Copy(file, readCloser)
	if err != nil {
		// Temporary file has been created if we go here. And some problems occurs such as network interruption and
		// so on. So we close and remove temporary file first to prevent residual file.
		closeAndRemoveFile(file, logger)
		return nil, errors.Wrap(err, "error copying Backup to temp file")
	}

	log := logger.WithField("backup", backupName)

	log.WithFields(logrus.Fields{
		"fileName": file.Name(),
		"bytes":    n,
	}).Debug("Copied Backup to file")

	if _, err := file.Seek(0, 0); err != nil {
		closeAndRemoveFile(file, logger)
		return nil, errors.Wrap(err, "error resetting Backup file offset")
	}

	return file, nil
}

type restoreLogger struct {
	logrus.FieldLogger
	file *os.File
	w    *gzip.Writer
}

func newRestoreLogger(restore *api.Restore, logLevel logrus.Level, logFormat logging.Format) (*restoreLogger, error) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "error creating temp file")
	}
	w := gzip.NewWriter(file)

	logger := logging.DefaultLogger(logLevel, logFormat)
	logger.Out = io.MultiWriter(os.Stdout, w)

	return &restoreLogger{
		FieldLogger: logger.WithField("restore", kubeutil.NamespaceAndName(restore)),
		file:        file,
		w:           w,
	}, nil
}

// done stops the restoreLogger from being able to be written to, and returns
// an io.Reader for getting the content of the logger. Any attempts to use
// restoreLogger to log after calling done will panic.
func (l *restoreLogger) done(log logrus.FieldLogger) (io.Reader, error) {
	l.FieldLogger = nil

	if err := l.w.Close(); err != nil {
		log.WithError(errors.WithStack(err)).Error("error closing gzip writer")
	}

	if _, err := l.file.Seek(0, 0); err != nil {
		return nil, errors.Wrap(err, "error resetting log file offset to 0")
	}

	return l.file, nil
}

// closeAndRemove removes the logger's underlying temporary storage. This
// method should be called when all logging and reading from the logger is
// complete.
func (l *restoreLogger) closeAndRemove(log logrus.FieldLogger) {
	closeAndRemoveFile(l.file, log)
}
