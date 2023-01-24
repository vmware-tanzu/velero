/*
Copyright the Velero contributors.

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
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	defaultAsyncBackupOperationsFrequency = 2 * time.Minute
)

type operationsForBackup struct {
	operations         []*itemoperation.BackupOperation
	changesSinceUpdate bool
	errsSinceUpdate    []string
}

// FIXME: remove if handled by backup finalizer controller
func (o *operationsForBackup) anyItemsToUpdate() bool {
	for _, op := range o.operations {
		if len(op.Spec.ItemsToUpdate) > 0 {
			return true
		}
	}
	return false
}
func (in *operationsForBackup) DeepCopy() *operationsForBackup {
	if in == nil {
		return nil
	}
	out := new(operationsForBackup)
	in.DeepCopyInto(out)
	return out
}

func (in *operationsForBackup) DeepCopyInto(out *operationsForBackup) {
	*out = *in
	if in.operations != nil {
		in, out := &in.operations, &out.operations
		*out = make([]*itemoperation.BackupOperation, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(itemoperation.BackupOperation)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.errsSinceUpdate != nil {
		in, out := &in.errsSinceUpdate, &out.errsSinceUpdate
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (o *operationsForBackup) uploadProgress(backupStore persistence.BackupStore, backupName string) error {
	if len(o.operations) > 0 {
		var backupItemOperations *bytes.Buffer
		backupItemOperations, errs := encodeToJSONGzip(o.operations, "backup item operations list")
		if errs != nil {
			return errors.Wrap(errs[0], "error encoding item operations json")
		}
		err := backupStore.PutBackupItemOperations(backupName, backupItemOperations)
		if err != nil {
			return errors.Wrap(err, "error uploading item operations json")
		}
	}
	o.changesSinceUpdate = false
	o.errsSinceUpdate = nil
	return nil
}

type BackupItemOperationsMap struct {
	operations map[string]*operationsForBackup
	opsLock    sync.Mutex
}

// If backup has changes not yet uploaded, upload them now
func (m *BackupItemOperationsMap) UpdateForBackup(backupStore persistence.BackupStore, backupName string) error {
	// lock operations map
	m.opsLock.Lock()
	defer m.opsLock.Unlock()

	operations, ok := m.operations[backupName]
	// if operations for this backup aren't found, or if there are no changes
	// or errors since last update, do nothing
	if !ok || (!operations.changesSinceUpdate && len(operations.errsSinceUpdate) == 0) {
		return nil
	}
	if err := operations.uploadProgress(backupStore, backupName); err != nil {
		return err
	}
	return nil
}

type asyncBackupOperationsReconciler struct {
	client.Client
	logger            logrus.FieldLogger
	clock             clocks.WithTickerAndDelayedExecution
	frequency         time.Duration
	itemOperationsMap *BackupItemOperationsMap
	newPluginManager  func(logger logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter
	metrics           *metrics.ServerMetrics
}

func NewAsyncBackupOperationsReconciler(
	logger logrus.FieldLogger,
	client client.Client,
	frequency time.Duration,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	metrics *metrics.ServerMetrics,
) (*asyncBackupOperationsReconciler, *BackupItemOperationsMap) {
	abor := &asyncBackupOperationsReconciler{
		Client:            client,
		logger:            logger,
		clock:             clocks.RealClock{},
		frequency:         frequency,
		itemOperationsMap: &BackupItemOperationsMap{operations: make(map[string]*operationsForBackup)},
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
		metrics:           metrics,
	}
	if abor.frequency <= 0 {
		abor.frequency = defaultAsyncBackupOperationsFrequency
	}
	return abor, abor.itemOperationsMap
}

func (c *asyncBackupOperationsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(c.logger, mgr.GetClient(), &velerov1api.BackupList{}, c.frequency, kube.PeriodicalEnqueueSourceOption{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Backup{}, builder.WithPredicates(kube.FalsePredicate{})).
		Watches(s, nil).
		Complete(c)
}

// +kubebuilder:rbac:groups=velero.io,resources=backups,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=velero.io,resources=backups/status,verbs=get
// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations,verbs=get
func (c *asyncBackupOperationsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.logger.WithField("async backup operations for backup", req.String())
	// FIXME: make this log.Debug
	log.Info("asyncBackupOperationsReconciler getting backup")

	original := &velerov1api.Backup{}
	if err := c.Get(ctx, req.NamespacedName, original); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Error("backup not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "error getting backup %s", req.String())
	}
	backup := original.DeepCopy()
	log.Debugf("backup: %s", backup.Name)

	log = c.logger.WithFields(
		logrus.Fields{
			"backup": req.String(),
		},
	)

	switch backup.Status.Phase {
	case velerov1api.BackupPhaseWaitingForPluginOperations, velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed:
		// only process backups waiting for plugin operations to complete
	default:
		log.Debug("Backup has no ongoing async plugin operations, skipping")
		return ctrl.Result{}, nil
	}

	loc := &velerov1api.BackupStorageLocation{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      backup.Spec.StorageLocation,
	}, loc); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf("Cannot check progress on async Backup operations because backup storage location %s does not exist; marking backup PartiallyFailed", backup.Spec.StorageLocation)
			backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
		} else {
			log.Warnf("Cannot check progress on async Backup operations because backup storage location %s could not be retrieved: %s; marking backup PartiallyFailed", backup.Spec.StorageLocation, err.Error())
			backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
		}
		err2 := c.updateBackupAndOperationsJSON(ctx, original, backup, nil, &operationsForBackup{errsSinceUpdate: []string{err.Error()}}, false, false)
		if err2 != nil {
			log.WithError(err2).Error("error updating Backup")
		}
		return ctrl.Result{}, errors.Wrap(err, "error getting backup storage location")
	}

	if loc.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
		log.Infof("Cannot check progress on async Backup operations because backup storage location %s is currently in read-only mode; marking backup PartiallyFailed", loc.Name)
		backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed

		err := c.updateBackupAndOperationsJSON(ctx, original, backup, nil, &operationsForBackup{errsSinceUpdate: []string{"BSL is read-only"}}, false, false)
		if err != nil {
			log.WithError(err).Error("error updating Backup")
		}
		return ctrl.Result{}, nil
	}

	pluginManager := c.newPluginManager(c.logger)
	defer pluginManager.CleanupClients()
	backupStore, err := c.backupStoreGetter.Get(loc, pluginManager, c.logger)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error getting backup store")
	}

	operations, err := c.getOperationsForBackup(backupStore, backup.Name)
	if err != nil {
		err2 := c.updateBackupAndOperationsJSON(ctx, original, backup, backupStore, &operationsForBackup{errsSinceUpdate: []string{err.Error()}}, false, false)
		if err2 != nil {
			return ctrl.Result{}, errors.Wrap(err2, "error updating Backup")
		}
		return ctrl.Result{}, errors.Wrap(err, "error getting backup operations")
	}
	stillInProgress, changes, opsCompleted, opsFailed, errs := getBackupItemOperationProgress(backup, pluginManager, operations.operations)
	// if len(errs)>0, need to update backup errors and error log
	operations.errsSinceUpdate = append(operations.errsSinceUpdate, errs...)
	backup.Status.Errors += len(operations.errsSinceUpdate)
	asyncCompletionChanges := false
	if backup.Status.AsyncBackupItemOperationsCompleted != opsCompleted || backup.Status.AsyncBackupItemOperationsFailed != opsFailed {
		asyncCompletionChanges = true
		backup.Status.AsyncBackupItemOperationsCompleted = opsCompleted
		backup.Status.AsyncBackupItemOperationsFailed = opsFailed
	}
	if changes {
		operations.changesSinceUpdate = true
	}

	// if stillInProgress is false, backup moves to finalize phase and needs update
	// if operations.errsSinceUpdate is not empty, then backup phase needs to change to
	// BackupPhaseWaitingForPluginOperationsPartiallyFailed and needs update
	// If the only changes are incremental progress, then no write is necessary, progress can remain in memory
	if !stillInProgress {
		if len(operations.errsSinceUpdate) > 0 {
			backup.Status.Phase = velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed
		}
		if backup.Status.Phase == velerov1api.BackupPhaseWaitingForPluginOperations {
			log.Infof("Marking backup %s FinalizingAfterPluginOperations", backup.Name)
			backup.Status.Phase = velerov1api.BackupPhaseFinalizingAfterPluginOperations
		} else {
			log.Infof("Marking backup %s FinalizingAfterPluginOperationsPartiallyFailed", backup.Name)
			backup.Status.Phase = velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed
		}
	}
	err = c.updateBackupAndOperationsJSON(ctx, original, backup, backupStore, operations, asyncCompletionChanges, changes)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error updating Backup")
	}
	return ctrl.Result{}, nil
}

func (c *asyncBackupOperationsReconciler) updateBackupAndOperationsJSON(
	ctx context.Context,
	original, backup *velerov1api.Backup,
	backupStore persistence.BackupStore,
	operations *operationsForBackup,
	changes bool,
	asyncCompletionChanges bool) error {

	backupScheduleName := backup.GetLabels()[velerov1api.ScheduleNameLabel]

	if len(operations.errsSinceUpdate) > 0 {
		c.metrics.RegisterBackupItemsErrorsGauge(backupScheduleName, backup.Status.Errors)
		// FIXME: download/upload results once https://github.com/vmware-tanzu/velero/pull/5576 is merged
	}
	removeIfComplete := true
	defer func() {
		// remove local operations list if complete
		c.itemOperationsMap.opsLock.Lock()
		if removeIfComplete && (backup.Status.Phase == velerov1api.BackupPhaseCompleted ||
			backup.Status.Phase == velerov1api.BackupPhasePartiallyFailed ||
			backup.Status.Phase == velerov1api.BackupPhaseFinalizingAfterPluginOperations ||
			backup.Status.Phase == velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed) {

			c.deleteOperationsForBackup(backup.Name)
		} else if changes {
			c.putOperationsForBackup(operations, backup.Name)
		}
		c.itemOperationsMap.opsLock.Unlock()
	}()

	// update backup and upload progress if errs or complete
	if len(operations.errsSinceUpdate) > 0 ||
		backup.Status.Phase == velerov1api.BackupPhaseCompleted ||
		backup.Status.Phase == velerov1api.BackupPhasePartiallyFailed ||
		backup.Status.Phase == velerov1api.BackupPhaseFinalizingAfterPluginOperations ||
		backup.Status.Phase == velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed {
		// update file store
		if backupStore != nil {
			backupJSON := new(bytes.Buffer)
			if err := encode.EncodeTo(backup, "json", backupJSON); err != nil {
				removeIfComplete = false
				return errors.Wrap(err, "error encoding backup json")
			}
			err := backupStore.PutBackupMetadata(backup.Name, backupJSON)
			if err != nil {
				removeIfComplete = false
				return errors.Wrap(err, "error uploading backup json")
			}
			if err := operations.uploadProgress(backupStore, backup.Name); err != nil {
				removeIfComplete = false
				return err
			}
		}
		// update backup
		err := c.Client.Patch(ctx, backup, client.MergeFrom(original))
		if err != nil {
			removeIfComplete = false
			return errors.Wrapf(err, "error updating Backup %s", backup.Name)
		}
	} else if asyncCompletionChanges {
		// If backup is still incomplete and no new errors are found but there are some new operations
		// completed, patch backup to reflect new completion numbers, but don't upload detailed json file
		err := c.Client.Patch(ctx, backup, client.MergeFrom(original))
		if err != nil {
			return errors.Wrapf(err, "error updating Backup %s", backup.Name)
		}
	}
	return nil
}

// returns a deep copy so we can minimize the time the map is locked
func (c *asyncBackupOperationsReconciler) getOperationsForBackup(
	backupStore persistence.BackupStore,
	backupName string) (*operationsForBackup, error) {
	var err error
	// lock operations map
	c.itemOperationsMap.opsLock.Lock()
	defer c.itemOperationsMap.opsLock.Unlock()

	operations, ok := c.itemOperationsMap.operations[backupName]
	if !ok || len(operations.operations) == 0 {
		operations = &operationsForBackup{}
		operations.operations, err = backupStore.GetBackupItemOperations(backupName)
		if err == nil {
			c.itemOperationsMap.operations[backupName] = operations
		}
	}
	return operations.DeepCopy(), err
}

func (c *asyncBackupOperationsReconciler) putOperationsForBackup(
	operations *operationsForBackup,
	backupName string) {
	if operations != nil {
		c.itemOperationsMap.operations[backupName] = operations
	}
}

func (c *asyncBackupOperationsReconciler) deleteOperationsForBackup(backupName string) {
	if _, ok := c.itemOperationsMap.operations[backupName]; ok {
		delete(c.itemOperationsMap.operations, backupName)
	}
	return
}

func getBackupItemOperationProgress(
	backup *velerov1api.Backup,
	pluginManager clientmgmt.Manager,
	operationsList []*itemoperation.BackupOperation) (bool, bool, int, int, []string) {
	inProgressOperations := false
	changes := false
	var errs []string
	var completedCount, failedCount int

	for _, operation := range operationsList {
		if operation.Status.Phase == itemoperation.OperationPhaseInProgress {
			bia, err := pluginManager.GetBackupItemActionV2(operation.Spec.BackupItemAction)
			if err != nil {
				operation.Status.Phase = itemoperation.OperationPhaseFailed
				operation.Status.Error = err.Error()
				errs = append(errs, err.Error())
				changes = true
				failedCount++
				continue
			}
			operationProgress, err := bia.Progress(operation.Spec.OperationID, backup)
			if err != nil {
				operation.Status.Phase = itemoperation.OperationPhaseFailed
				operation.Status.Error = err.Error()
				errs = append(errs, err.Error())
				changes = true
				failedCount++
				continue
			}
			if operation.Status.NCompleted != operationProgress.NCompleted {
				operation.Status.NCompleted = operationProgress.NCompleted
				changes = true
			}
			if operation.Status.NTotal != operationProgress.NTotal {
				operation.Status.NTotal = operationProgress.NTotal
				changes = true
			}
			if operation.Status.OperationUnits != operationProgress.OperationUnits {
				operation.Status.OperationUnits = operationProgress.OperationUnits
				changes = true
			}
			if operation.Status.Description != operationProgress.Description {
				operation.Status.Description = operationProgress.Description
				changes = true
			}
			started := metav1.NewTime(operationProgress.Started)
			if operation.Status.Started == nil || *(operation.Status.Started) != started {
				operation.Status.Started = &started
				changes = true
			}
			updated := metav1.NewTime(operationProgress.Updated)
			if operation.Status.Updated == nil || *(operation.Status.Updated) != updated {
				operation.Status.Updated = &updated
				changes = true
			}

			if operationProgress.Completed {
				if operationProgress.Err != "" {
					operation.Status.Phase = itemoperation.OperationPhaseFailed
					operation.Status.Error = operationProgress.Err
					errs = append(errs, operationProgress.Err)
					changes = true
					failedCount++
					continue
				}
				operation.Status.Phase = itemoperation.OperationPhaseCompleted
				changes = true
				completedCount++
				continue
			}
			// cancel operation if past timeout period
			if operation.Status.Created.Time.Add(backup.Spec.ItemOperationTimeout.Duration).Before(time.Now()) {
				_ = bia.Cancel(operation.Spec.OperationID, backup)
				operation.Status.Phase = itemoperation.OperationPhaseFailed
				operation.Status.Error = "Asynchronous action timed out"
				errs = append(errs, operation.Status.Error)
				changes = true
				failedCount++
				continue
			}
			// if we reach this point, the operation is still running
			inProgressOperations = true
		} else if operation.Status.Phase == itemoperation.OperationPhaseCompleted {
			completedCount++
		} else if operation.Status.Phase == itemoperation.OperationPhaseFailed {
			failedCount++
		}
	}
	return inProgressOperations, changes, completedCount, failedCount, errs
}
