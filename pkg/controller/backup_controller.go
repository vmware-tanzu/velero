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
	"fmt"
	"os"
	"sync"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotterClientSet "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	snapshotv1listers "github.com/kubernetes-csi/external-snapshotter/client/v4/listers/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1api "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	"github.com/vmware-tanzu/velero/pkg/util/results"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

const (
	backupResyncPeriod = time.Minute
)

type backupReconciler struct {
	ctx                         context.Context
	logger                      logrus.FieldLogger
	discoveryHelper             discovery.Helper
	backupper                   pkgbackup.Backupper
	kbClient                    kbclient.Client
	clock                       clock.WithTickerAndDelayedExecution
	backupLogLevel              logrus.Level
	newPluginManager            func(logrus.FieldLogger) clientmgmt.Manager
	backupTracker               BackupTracker
	defaultBackupLocation       string
	defaultVolumesToFsBackup    bool
	defaultBackupTTL            time.Duration
	defaultCSISnapshotTimeout   time.Duration
	resourceTimeout             time.Duration
	defaultItemOperationTimeout time.Duration
	defaultSnapshotLocations    map[string]string
	metrics                     *metrics.ServerMetrics
	backupStoreGetter           persistence.ObjectBackupStoreGetter
	formatFlag                  logging.Format
	volumeSnapshotLister        snapshotv1listers.VolumeSnapshotLister
	volumeSnapshotClient        snapshotterClientSet.Interface
	credentialFileStore         credentials.FileStore
	maxConcurrentK8SConnections int
}

func NewBackupReconciler(
	ctx context.Context,
	discoveryHelper discovery.Helper,
	backupper pkgbackup.Backupper,
	logger logrus.FieldLogger,
	backupLogLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupTracker BackupTracker,
	kbClient kbclient.Client,
	defaultBackupLocation string,
	defaultVolumesToFsBackup bool,
	defaultBackupTTL time.Duration,
	defaultCSISnapshotTimeout time.Duration,
	resourceTimeout time.Duration,
	defaultItemOperationTimeout time.Duration,
	defaultSnapshotLocations map[string]string,
	metrics *metrics.ServerMetrics,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	formatFlag logging.Format,
	volumeSnapshotLister snapshotv1listers.VolumeSnapshotLister,
	volumeSnapshotClient snapshotterClientSet.Interface,
	credentialStore credentials.FileStore,
	maxConcurrentK8SConnections int,
) *backupReconciler {

	b := &backupReconciler{
		ctx:                         ctx,
		discoveryHelper:             discoveryHelper,
		backupper:                   backupper,
		clock:                       &clock.RealClock{},
		logger:                      logger,
		backupLogLevel:              backupLogLevel,
		newPluginManager:            newPluginManager,
		backupTracker:               backupTracker,
		kbClient:                    kbClient,
		defaultBackupLocation:       defaultBackupLocation,
		defaultVolumesToFsBackup:    defaultVolumesToFsBackup,
		defaultBackupTTL:            defaultBackupTTL,
		defaultCSISnapshotTimeout:   defaultCSISnapshotTimeout,
		resourceTimeout:             resourceTimeout,
		defaultItemOperationTimeout: defaultItemOperationTimeout,
		defaultSnapshotLocations:    defaultSnapshotLocations,
		metrics:                     metrics,
		backupStoreGetter:           backupStoreGetter,
		formatFlag:                  formatFlag,
		volumeSnapshotLister:        volumeSnapshotLister,
		volumeSnapshotClient:        volumeSnapshotClient,
		credentialFileStore:         credentialStore,
		maxConcurrentK8SConnections: maxConcurrentK8SConnections,
	}
	b.updateTotalBackupMetric()
	return b
}

func (b *backupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Backup{}).
		Complete(b)
}

func (b *backupReconciler) updateTotalBackupMetric() {
	go func() {
		// Wait for 5 seconds to let controller-runtime to setup k8s clients.
		time.Sleep(5 * time.Second)

		wait.Until(
			func() {
				// recompute backup_total metric
				backups := &velerov1api.BackupList{}
				err := b.kbClient.List(context.Background(), backups, &kbclient.ListOptions{LabelSelector: labels.Everything()})
				if err != nil {
					b.logger.Error(err, "Error computing backup_total metric")
				} else {
					b.metrics.SetBackupTotal(int64(len(backups.Items)))
				}

				// recompute backup_last_successful_timestamp metric for each
				// schedule (including the empty schedule, i.e. ad-hoc backups)
				for schedule, timestamp := range getLastSuccessBySchedule(backups.Items) {
					b.metrics.SetBackupLastSuccessfulTimestamp(schedule, timestamp)
				}
			},
			backupResyncPeriod,
			b.ctx.Done(),
		)
	}()
}

// getLastSuccessBySchedule finds the most recent completed backup for each schedule
// and returns a map of schedule name -> completion time of the most recent completed
// backup. This map includes an entry for ad-hoc/non-scheduled backups, where the key
// is the empty string.
func getLastSuccessBySchedule(backups []velerov1api.Backup) map[string]time.Time {
	lastSuccessBySchedule := map[string]time.Time{}
	for _, backup := range backups {
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted {
			continue
		}
		if backup.Status.CompletionTimestamp == nil {
			continue
		}

		schedule := backup.Labels[velerov1api.ScheduleNameLabel]
		timestamp := backup.Status.CompletionTimestamp.Time

		if timestamp.After(lastSuccessBySchedule[schedule]) {
			lastSuccessBySchedule[schedule] = timestamp
		}
	}

	return lastSuccessBySchedule
}

func (b *backupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := b.logger.WithFields(logrus.Fields{
		"controller":    Backup,
		"backuprequest": req.String(),
	})

	log.Debug("Getting backup")

	original := &velerov1api.Backup{}
	err := b.kbClient.Get(ctx, req.NamespacedName, original)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("backup not found")
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("error getting backup")
		return ctrl.Result{}, err
	}

	// Double-check we have the correct phase. In the unlikely event that multiple controller
	// instances are running, it's possible for controller A to succeed in changing the phase to
	// InProgress, while controller B's attempt to patch the phase fails. When controller B
	// reprocesses the same backup, it will either show up as New (informer hasn't seen the update
	// yet) or as InProgress. In the former case, the patch attempt will fail again, until the
	// informer sees the update. In the latter case, after the informer has seen the update to
	// InProgress, we still need this check so we can return nil to indicate we've finished processing
	// this key (even though it was a no-op).
	switch original.Status.Phase {
	case "", velerov1api.BackupPhaseNew:
		// only process new backups
	default:
		b.logger.WithFields(logrus.Fields{
			"backup": kubeutil.NamespaceAndName(original),
			"phase":  original.Status.Phase,
		}).Debug("Backup is not handled")
		return ctrl.Result{}, nil
	}

	log.Debug("Preparing backup request")
	request := b.prepareBackupRequest(original, log)
	if len(request.Status.ValidationErrors) > 0 {
		request.Status.Phase = velerov1api.BackupPhaseFailedValidation
	} else {
		request.Status.Phase = velerov1api.BackupPhaseInProgress
		request.Status.StartTimestamp = &metav1.Time{Time: b.clock.Now()}
	}

	// update status
	if err := kubeutil.PatchResource(original, request.Backup, b.kbClient); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error updating Backup status to %s", request.Status.Phase)
	}
	// store ref to just-updated item for creating patch
	original = request.Backup.DeepCopy()

	if request.Status.Phase == velerov1api.BackupPhaseFailedValidation {
		log.Debug("failed to validate backup status")
		return ctrl.Result{}, nil
	}

	b.backupTracker.Add(request.Namespace, request.Name)
	defer func() {
		switch request.Status.Phase {
		case velerov1api.BackupPhaseCompleted, velerov1api.BackupPhasePartiallyFailed, velerov1api.BackupPhaseFailed, velerov1api.BackupPhaseFailedValidation:
			b.backupTracker.Delete(request.Namespace, request.Name)
		}
	}()

	log.Debug("Running backup")

	backupScheduleName := request.GetLabels()[velerov1api.ScheduleNameLabel]
	b.metrics.RegisterBackupAttempt(backupScheduleName)

	// execution & upload of backup
	if err := b.runBackup(request); err != nil {
		// even though runBackup sets the backup's phase prior
		// to uploading artifacts to object storage, we have to
		// check for an error again here and update the phase if
		// one is found, because there could've been an error
		// while uploading artifacts to object storage, which would
		// result in the backup being Failed.
		log.WithError(err).Error("backup failed")
		request.Status.Phase = velerov1api.BackupPhaseFailed
		request.Status.FailureReason = err.Error()
	}

	switch request.Status.Phase {
	case velerov1api.BackupPhaseCompleted:
		b.metrics.RegisterBackupSuccess(backupScheduleName)
		b.metrics.RegisterBackupLastStatus(backupScheduleName, metrics.BackupLastStatusSucc)
	case velerov1api.BackupPhasePartiallyFailed:
		b.metrics.RegisterBackupPartialFailure(backupScheduleName)
		b.metrics.RegisterBackupLastStatus(backupScheduleName, metrics.BackupLastStatusFailure)
	case velerov1api.BackupPhaseFailed:
		b.metrics.RegisterBackupFailed(backupScheduleName)
		b.metrics.RegisterBackupLastStatus(backupScheduleName, metrics.BackupLastStatusFailure)
	case velerov1api.BackupPhaseFailedValidation:
		b.metrics.RegisterBackupValidationFailure(backupScheduleName)
		b.metrics.RegisterBackupLastStatus(backupScheduleName, metrics.BackupLastStatusFailure)
	}
	log.Info("Updating backup's final status")
	if err := kubeutil.PatchResource(original, request.Backup, b.kbClient); err != nil {
		log.WithError(err).Error("error updating backup's final status")
	}
	return ctrl.Result{}, nil
}

func (b *backupReconciler) prepareBackupRequest(backup *velerov1api.Backup, logger logrus.FieldLogger) *pkgbackup.Request {
	request := &pkgbackup.Request{
		Backup: backup.DeepCopy(), // don't modify items in the cache
	}

	// set backup major version - deprecated, use Status.FormatVersion
	request.Status.Version = pkgbackup.BackupVersion

	// set backup major, minor, and patch version
	request.Status.FormatVersion = pkgbackup.BackupFormatVersion

	if request.Spec.TTL.Duration == 0 {
		// set default backup TTL
		request.Spec.TTL.Duration = b.defaultBackupTTL
	}

	if request.Spec.CSISnapshotTimeout.Duration == 0 {
		// set default CSI VolumeSnapshot timeout
		request.Spec.CSISnapshotTimeout.Duration = b.defaultCSISnapshotTimeout
	}

	if request.Spec.ItemOperationTimeout.Duration == 0 {
		// set default item operation timeout
		request.Spec.ItemOperationTimeout.Duration = b.defaultItemOperationTimeout
	}

	// calculate expiration
	request.Status.Expiration = &metav1.Time{Time: b.clock.Now().Add(request.Spec.TTL.Duration)}

	// TODO: post v1.10. Remove this code block after DefaultVolumesToRestic is removed from CRD
	// For now, for CRs created by old versions, we need to respect the DefaultVolumesToRestic value if it is set true
	if boolptr.IsSetToTrue(request.Spec.DefaultVolumesToRestic) {
		logger.Warn("DefaultVolumesToRestic field will be deprecated, use DefaultVolumesToFsBackup instead. Automatically remap it to DefaultVolumesToFsBackup")
		request.Spec.DefaultVolumesToFsBackup = request.Spec.DefaultVolumesToRestic
	}

	if request.Spec.DefaultVolumesToFsBackup == nil {
		request.Spec.DefaultVolumesToFsBackup = &b.defaultVolumesToFsBackup
	}

	// find which storage location to use
	var serverSpecified bool
	if request.Spec.StorageLocation == "" {
		// when the user doesn't specify a location, use the server default unless there is an existing BSL marked as default
		// TODO(2.0) b.defaultBackupLocation will be deprecated
		request.Spec.StorageLocation = b.defaultBackupLocation

		locationList, err := storage.ListBackupStorageLocations(context.Background(), b.kbClient, request.Namespace)
		if err == nil {
			for _, location := range locationList.Items {
				if location.Spec.Default {
					request.Spec.StorageLocation = location.Name
					break
				}
			}
		}
		serverSpecified = true
	}

	// get the storage location, and store the BackupStorageLocation API obj on the request
	storageLocation := &velerov1api.BackupStorageLocation{}
	if err := b.kbClient.Get(context.Background(), kbclient.ObjectKey{
		Namespace: request.Namespace,
		Name:      request.Spec.StorageLocation,
	}, storageLocation); err != nil {
		if apierrors.IsNotFound(err) {
			if serverSpecified {
				// TODO(2.0) remove this. For now, without mentioning "server default" it could be confusing trying to grasp where the default came from.
				request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("an existing backup storage location wasn't specified at backup creation time and the server default '%s' doesn't exist. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: %v", request.Spec.StorageLocation, err))
			} else {
				request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("an existing backup storage location wasn't specified at backup creation time and the default '%s' wasn't found. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: %v", request.Spec.StorageLocation, err))
			}
		} else {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("error getting backup storage location: %v", err))
		}
	} else {
		request.StorageLocation = storageLocation

		if request.StorageLocation.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors,
				fmt.Sprintf("backup can't be created because backup storage location %s is currently in read-only mode", request.StorageLocation.Name))
		}
	}

	// add the storage location as a label for easy filtering later.
	if request.Labels == nil {
		request.Labels = make(map[string]string)
	}
	request.Labels[velerov1api.StorageLocationLabel] = label.GetValidName(request.Spec.StorageLocation)

	// validate and get the backup's VolumeSnapshotLocations, and store the
	// VolumeSnapshotLocation API objs on the request
	if locs, errs := b.validateAndGetSnapshotLocations(request.Backup); len(errs) > 0 {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, errs...)
	} else {
		request.Spec.VolumeSnapshotLocations = nil
		for _, loc := range locs {
			request.Spec.VolumeSnapshotLocations = append(request.Spec.VolumeSnapshotLocations, loc.Name)
			request.SnapshotLocations = append(request.SnapshotLocations, loc)
		}
	}

	// Getting all information of cluster version - useful for future skip-level migration
	if request.Annotations == nil {
		request.Annotations = make(map[string]string)
	}
	request.Annotations[velerov1api.SourceClusterK8sGitVersionAnnotation] = b.discoveryHelper.ServerVersion().String()
	request.Annotations[velerov1api.SourceClusterK8sMajorVersionAnnotation] = b.discoveryHelper.ServerVersion().Major
	request.Annotations[velerov1api.SourceClusterK8sMinorVersionAnnotation] = b.discoveryHelper.ServerVersion().Minor

	// Add namespaces with label velero.io/exclude-from-backup=true into request.Spec.ExcludedNamespaces
	// Essentially, adding the label velero.io/exclude-from-backup=true to a namespace would be equivalent to setting spec.ExcludedNamespaces
	namespaces := corev1api.NamespaceList{}
	if err := b.kbClient.List(context.Background(), &namespaces, kbclient.MatchingLabels{"velero.io/exclude-from-backup": "true"}); err == nil {
		for _, ns := range namespaces.Items {
			request.Spec.ExcludedNamespaces = append(request.Spec.ExcludedNamespaces, ns.Name)
		}
	} else {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("error getting namespace list: %v", err))
	}

	// validate whether Included/Excluded resources and IncludedClusterResource are mixed with
	// Included/Excluded cluster-scoped/namespace-scoped resources.
	if oldAndNewFilterParametersUsedTogether(request.Spec) {
		validatedError := fmt.Sprintf("include-resources, exclude-resources and include-cluster-resources are old filter parameters.\n" +
			"include-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources are new filter parameters.\n" +
			"They cannot be used together")
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, validatedError)
	}

	// validate the included/excluded resources
	for _, err := range collections.ValidateIncludesExcludes(request.Spec.IncludedResources, request.Spec.ExcludedResources) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	// validate the cluster-scoped included/excluded resources
	for _, err := range collections.ValidateScopedIncludesExcludes(request.Spec.IncludedClusterScopedResources, request.Spec.ExcludedClusterScopedResources) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid cluster-scoped included/excluded resource lists: %s", err))
	}

	// validate the namespace-scoped included/excluded resources
	for _, err := range collections.ValidateScopedIncludesExcludes(request.Spec.IncludedNamespaceScopedResources, request.Spec.ExcludedNamespaceScopedResources) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid namespace-scoped included/excluded resource lists: %s", err))
	}

	// validate the included/excluded namespaces
	for _, err := range collections.ValidateNamespaceIncludesExcludes(request.Spec.IncludedNamespaces, request.Spec.ExcludedNamespaces) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	// validate that only one exists orLabelSelector or just labelSelector (singular)
	if request.Spec.OrLabelSelectors != nil && request.Spec.LabelSelector != nil {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("encountered labelSelector as well as orLabelSelectors in backup spec, only one can be specified"))
	}

	if request.Spec.ResourcePolicy != nil && request.Spec.ResourcePolicy.Kind == resourcepolicies.ConfigmapRefType {
		policiesConfigmap := &v1.ConfigMap{}
		err := b.kbClient.Get(context.Background(), kbclient.ObjectKey{Namespace: request.Namespace, Name: request.Spec.ResourcePolicy.Name}, policiesConfigmap)
		if err != nil {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("failed to get resource policies %s/%s configmap with err %v", request.Namespace, request.Spec.ResourcePolicy.Name, err))
		}
		res, err := resourcepolicies.GetResourcePoliciesFromConfig(policiesConfigmap)
		if err != nil {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, errors.Wrapf(err, fmt.Sprintf("resource policies %s/%s", request.Namespace, request.Spec.ResourcePolicy.Name)).Error())
		} else if err = res.Validate(); err != nil {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, errors.Wrapf(err, fmt.Sprintf("resource policies %s/%s", request.Namespace, request.Spec.ResourcePolicy.Name)).Error())
		}
		request.ResPolicies = res
	}

	return request
}

// validateAndGetSnapshotLocations gets a collection of VolumeSnapshotLocation objects that
// this backup will use (returned as a map of provider name -> VSL), and ensures:
//   - each location name in .spec.volumeSnapshotLocations exists as a location
//   - exactly 1 location per provider
//   - a given provider's default location name is added to .spec.volumeSnapshotLocations if one
//     is not explicitly specified for the provider (if there's only one location for the provider,
//     it will automatically be used)
//
// if backup has snapshotVolume disabled then it returns empty VSL
func (b *backupReconciler) validateAndGetSnapshotLocations(backup *velerov1api.Backup) (map[string]*velerov1api.VolumeSnapshotLocation, []string) {
	errors := []string{}
	providerLocations := make(map[string]*velerov1api.VolumeSnapshotLocation)

	// if snapshotVolume is set to false then we don't need to validate volumesnapshotlocation
	if boolptr.IsSetToFalse(backup.Spec.SnapshotVolumes) {
		return nil, nil
	}

	for _, locationName := range backup.Spec.VolumeSnapshotLocations {
		// validate each locationName exists as a VolumeSnapshotLocation
		location := &velerov1api.VolumeSnapshotLocation{}
		if err := b.kbClient.Get(context.Background(), kbclient.ObjectKey{Namespace: backup.Namespace, Name: locationName}, location); err != nil {
			if apierrors.IsNotFound(err) {
				errors = append(errors, fmt.Sprintf("a VolumeSnapshotLocation CRD for the location %s with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: %v", locationName, err))
			} else {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", locationName, err))
			}
			continue
		}

		// ensure we end up with exactly 1 location *per provider*
		if providerLocation, ok := providerLocations[location.Spec.Provider]; ok {
			// if > 1 location name per provider as in ["aws-us-east-1" | "aws-us-west-1"] (same provider, multiple names)
			if providerLocation.Name != locationName {
				errors = append(errors, fmt.Sprintf("more than one VolumeSnapshotLocation name specified for provider %s: %s; unexpected name was %s", location.Spec.Provider, locationName, providerLocation.Name))
				continue
			}
		} else {
			// keep track of all valid existing locations, per provider
			providerLocations[location.Spec.Provider] = location
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}
	allLocations := &velerov1api.VolumeSnapshotLocationList{}
	err := b.kbClient.List(context.Background(), allLocations, &kbclient.ListOptions{Namespace: backup.Namespace, LabelSelector: labels.Everything()})
	if err != nil {
		errors = append(errors, fmt.Sprintf("error listing volume snapshot locations: %v", err))
		return nil, errors
	}

	// build a map of provider->list of all locations for the provider
	allProviderLocations := make(map[string][]*velerov1api.VolumeSnapshotLocation)
	for i := range allLocations.Items {
		loc := allLocations.Items[i]
		allProviderLocations[loc.Spec.Provider] = append(allProviderLocations[loc.Spec.Provider], &loc)
	}

	// go through each provider and make sure we have/can get a VSL
	// for it
	for provider, locations := range allProviderLocations {
		if _, ok := providerLocations[provider]; ok {
			// backup's spec had a location named for this provider
			continue
		}

		if len(locations) > 1 {
			// more than one possible location for the provider: check
			// the defaults
			defaultLocation := b.defaultSnapshotLocations[provider]
			if defaultLocation == "" {
				errors = append(errors, fmt.Sprintf("provider %s has more than one possible volume snapshot location, and none were specified explicitly or as a default", provider))
				continue
			}
			location := &velerov1api.VolumeSnapshotLocation{}
			b.kbClient.Get(context.Background(), kbclient.ObjectKey{Namespace: backup.Namespace, Name: defaultLocation}, location)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", defaultLocation, err))
				continue
			}

			providerLocations[provider] = location
			continue
		}

		// exactly one location for the provider: use it
		providerLocations[provider] = locations[0]
	}

	if len(errors) > 0 {
		return nil, errors
	}

	// add credential to config for each location
	for _, location := range providerLocations {
		err = volume.UpdateVolumeSnapshotLocationWithCredentialConfig(location, b.credentialFileStore)
		if err != nil {
			errors = append(errors, fmt.Sprintf("error adding credentials to volume snapshot location named %s: %v", location.Name, err))
			continue
		}
	}

	return providerLocations, nil
}

// runBackup runs and uploads a validated backup. Any error returned from this function
// causes the backup to be Failed; if no error is returned, the backup's status's Errors
// field is checked to see if the backup was a partial failure.

func (b *backupReconciler) runBackup(backup *pkgbackup.Request) error {
	b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)).Info("Setting up backup log")

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logCounter := logging.NewLogHook()
	backupLog, err := logging.NewTempFileLogger(b.backupLogLevel, b.formatFlag, logCounter, logrus.Fields{Backup: kubeutil.NamespaceAndName(backup)})
	if err != nil {
		return errors.Wrap(err, "error creating dual mode logger for backup")
	}
	defer backupLog.Dispose(b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)))

	backupLog.Info("Setting up backup temp file")
	backupFile, err := os.CreateTemp("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup")
	}
	defer closeAndRemoveFile(backupFile, backupLog)

	backupLog.Info("Setting up plugin manager")
	pluginManager := b.newPluginManager(backupLog)
	defer pluginManager.CleanupClients()

	backupLog.Info("Getting backup item actions")
	actions, err := pluginManager.GetBackupItemActionsV2()
	if err != nil {
		return err
	}
	backupLog.Info("Setting up backup store to check for backup existence")
	backupStore, err := b.backupStoreGetter.Get(backup.StorageLocation, pluginManager, backupLog)
	if err != nil {
		return err
	}

	exists, err := backupStore.BackupExists(backup.StorageLocation.Spec.StorageType.ObjectStorage.Bucket, backup.Name)
	if exists || err != nil {
		backup.Status.Phase = velerov1api.BackupPhaseFailed
		backup.Status.CompletionTimestamp = &metav1.Time{Time: b.clock.Now()}
		if err != nil {
			return errors.Wrapf(err, "error checking if backup already exists in object storage")
		}
		return errors.Errorf("backup already exists in object storage")
	}

	backupItemActionsResolver := framework.NewBackupItemActionResolverV2(actions)

	var fatalErrs []error
	if err := b.backupper.BackupWithResolvers(backupLog, backup, backupFile, backupItemActionsResolver, pluginManager); err != nil {
		fatalErrs = append(fatalErrs, err)
	}

	// Empty slices here so that they can be passed in to the persistBackup call later, regardless of whether or not CSI's enabled.
	// This way, we only make the Lister call if the feature flag's on.
	var volumeSnapshots []snapshotv1api.VolumeSnapshot
	var volumeSnapshotContents []snapshotv1api.VolumeSnapshotContent
	var volumeSnapshotClasses []snapshotv1api.VolumeSnapshotClass
	if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		selector := label.NewSelectorForBackup(backup.Name)
		vscList := &snapshotv1api.VolumeSnapshotContentList{}

		volumeSnapshots, err = b.waitVolumeSnapshotReadyToUse(context.Background(), backup.Spec.CSISnapshotTimeout.Duration, backup.Name)
		if err != nil {
			backupLog.Errorf("fail to wait VolumeSnapshot change to Ready: %s", err.Error())
		}

		backup.CSISnapshots = volumeSnapshots

		err = b.kbClient.List(context.Background(), vscList, &kbclient.ListOptions{LabelSelector: selector})
		if err != nil {
			backupLog.Error(err)
		}
		if len(vscList.Items) >= 0 {
			volumeSnapshotContents = vscList.Items
		}

		vsClassSet := sets.NewString()
		for index := range volumeSnapshotContents {
			// persist the volumesnapshotclasses referenced by vsc
			if volumeSnapshotContents[index].Spec.VolumeSnapshotClassName != nil && !vsClassSet.Has(*volumeSnapshotContents[index].Spec.VolumeSnapshotClassName) {
				vsClass := &snapshotv1api.VolumeSnapshotClass{}
				if err := b.kbClient.Get(context.TODO(), kbclient.ObjectKey{Name: *volumeSnapshotContents[index].Spec.VolumeSnapshotClassName}, vsClass); err != nil {
					backupLog.Error(err)
				} else {
					vsClassSet.Insert(*volumeSnapshotContents[index].Spec.VolumeSnapshotClassName)
					volumeSnapshotClasses = append(volumeSnapshotClasses, *vsClass)
				}
			}

			if err := csi.ResetVolumeSnapshotContent(&volumeSnapshotContents[index]); err != nil {
				backupLog.Error(err)
			}
		}

		// Delete the VolumeSnapshots created in the backup, when CSI feature is enabled.
		if len(volumeSnapshots) > 0 && len(volumeSnapshotContents) > 0 {
			b.deleteVolumeSnapshots(volumeSnapshots, volumeSnapshotContents, backupLog, b.maxConcurrentK8SConnections)
		}
	}

	backup.Status.VolumeSnapshotsAttempted = len(backup.VolumeSnapshots)
	for _, snap := range backup.VolumeSnapshots {
		if snap.Status.Phase == volume.SnapshotPhaseCompleted {
			backup.Status.VolumeSnapshotsCompleted++
		}
	}

	backup.Status.CSIVolumeSnapshotsAttempted = len(backup.CSISnapshots)
	for _, vs := range backup.CSISnapshots {
		if vs.Status != nil && boolptr.IsSetToTrue(vs.Status.ReadyToUse) {
			backup.Status.CSIVolumeSnapshotsCompleted++
		}
	}

	// Iterate over backup item operations and update progress.
	// Any errors on operations at this point should be added to backup errors.
	// If any operations are still not complete, then back will not be set to
	// Completed yet.
	inProgressOperations, _, opsCompleted, opsFailed, errs := getBackupItemOperationProgress(backup.Backup, pluginManager, *backup.GetItemOperationsList())
	if len(errs) > 0 {
		for err := range errs {
			backupLog.Error(err)
		}
	}

	backup.Status.BackupItemOperationsAttempted = len(*backup.GetItemOperationsList())
	backup.Status.BackupItemOperationsCompleted = opsCompleted
	backup.Status.BackupItemOperationsFailed = opsFailed

	backup.Status.Warnings = logCounter.GetCount(logrus.WarnLevel)
	backup.Status.Errors = logCounter.GetCount(logrus.ErrorLevel)

	backupWarnings := logCounter.GetEntries(logrus.WarnLevel)
	backupErrors := logCounter.GetEntries(logrus.ErrorLevel)
	results := map[string]results.Result{
		"warnings": backupWarnings,
		"errors":   backupErrors,
	}

	backupLog.DoneForPersist(b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)))

	// Assign finalize phase as close to end as possible so that any errors
	// logged to backupLog are captured. This is done before uploading the
	// artifacts to object storage so that the JSON representation of the
	// backup in object storage has the terminal phase set.
	switch {
	case len(fatalErrs) > 0:
		backup.Status.Phase = velerov1api.BackupPhaseFailed
	case logCounter.GetCount(logrus.ErrorLevel) > 0:
		if inProgressOperations {
			backup.Status.Phase = velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed
		} else {
			backup.Status.Phase = velerov1api.BackupPhaseFinalizingPartiallyFailed
		}
	default:
		if inProgressOperations {
			backup.Status.Phase = velerov1api.BackupPhaseWaitingForPluginOperations
		} else {
			backup.Status.Phase = velerov1api.BackupPhaseFinalizing
		}
	}
	// Mark completion timestamp before serializing and uploading.
	// Otherwise, the JSON file in object storage has a CompletionTimestamp of 'null'.
	if backup.Status.Phase == velerov1api.BackupPhaseFailed ||
		backup.Status.Phase == velerov1api.BackupPhasePartiallyFailed ||
		backup.Status.Phase == velerov1api.BackupPhaseCompleted {
		backup.Status.CompletionTimestamp = &metav1.Time{Time: b.clock.Now()}
	}
	recordBackupMetrics(backupLog, backup.Backup, backupFile, b.metrics, false)

	// re-instantiate the backup store because credentials could have changed since the original
	// instantiation, if this was a long-running backup
	backupLog.Info("Setting up backup store to persist the backup")
	backupStore, err = b.backupStoreGetter.Get(backup.StorageLocation, pluginManager, backupLog)
	if err != nil {
		return err
	}

	if logFile, err := backupLog.GetPersistFile(); err != nil {
		fatalErrs = append(fatalErrs, errors.Wrap(err, "error getting backup log file"))
	} else {
		if errs := persistBackup(backup, backupFile, logFile, backupStore, volumeSnapshots, volumeSnapshotContents, volumeSnapshotClasses, results); len(errs) > 0 {
			fatalErrs = append(fatalErrs, errs...)
		}
	}

	b.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)).Info("Backup completed")

	// if we return a non-nil error, the calling function will update
	// the backup's phase to Failed.
	return kerrors.NewAggregate(fatalErrs)
}

func recordBackupMetrics(log logrus.FieldLogger, backup *velerov1api.Backup, backupFile *os.File, serverMetrics *metrics.ServerMetrics, finalize bool) {
	backupScheduleName := backup.GetLabels()[velerov1api.ScheduleNameLabel]

	if backupFile != nil {
		var backupSizeBytes int64
		if backupFileStat, err := backupFile.Stat(); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error getting backup file info")
		} else {
			backupSizeBytes = backupFileStat.Size()
		}
		serverMetrics.SetBackupTarballSizeBytesGauge(backupScheduleName, backupSizeBytes)
	}

	if backup.Status.CompletionTimestamp != nil {
		backupDuration := backup.Status.CompletionTimestamp.Time.Sub(backup.Status.StartTimestamp.Time)
		backupDurationSeconds := float64(backupDuration / time.Second)
		serverMetrics.RegisterBackupDuration(backupScheduleName, backupDurationSeconds)
	}
	if !finalize {
		serverMetrics.RegisterVolumeSnapshotAttempts(backupScheduleName, backup.Status.VolumeSnapshotsAttempted)
		serverMetrics.RegisterVolumeSnapshotSuccesses(backupScheduleName, backup.Status.VolumeSnapshotsCompleted)
		serverMetrics.RegisterVolumeSnapshotFailures(backupScheduleName, backup.Status.VolumeSnapshotsAttempted-backup.Status.VolumeSnapshotsCompleted)

		if features.IsEnabled(velerov1api.CSIFeatureFlag) {
			serverMetrics.RegisterCSISnapshotAttempts(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsAttempted)
			serverMetrics.RegisterCSISnapshotSuccesses(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsCompleted)
			serverMetrics.RegisterCSISnapshotFailures(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsAttempted-backup.Status.CSIVolumeSnapshotsCompleted)
		}

		if backup.Status.Progress != nil {
			serverMetrics.RegisterBackupItemsTotalGauge(backupScheduleName, backup.Status.Progress.TotalItems)
		}
		serverMetrics.RegisterBackupItemsErrorsGauge(backupScheduleName, backup.Status.Errors)

		if backup.Status.Warnings > 0 {
			serverMetrics.RegisterBackupWarning(backupScheduleName)
		}
	}
}

func persistBackup(backup *pkgbackup.Request,
	backupContents, backupLog *os.File,
	backupStore persistence.BackupStore,
	csiVolumeSnapshots []snapshotv1api.VolumeSnapshot,
	csiVolumeSnapshotContents []snapshotv1api.VolumeSnapshotContent,
	csiVolumesnapshotClasses []snapshotv1api.VolumeSnapshotClass,
	results map[string]results.Result,
) []error {
	persistErrs := []error{}
	backupJSON := new(bytes.Buffer)

	if err := encode.EncodeTo(backup.Backup, "json", backupJSON); err != nil {
		persistErrs = append(persistErrs, errors.Wrap(err, "error encoding backup"))
	}

	// Velero-native volume snapshots (as opposed to CSI ones)
	nativeVolumeSnapshots, errs := encode.EncodeToJSONGzip(backup.VolumeSnapshots, "native volumesnapshots list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	var backupItemOperations *bytes.Buffer
	backupItemOperations, errs = encode.EncodeToJSONGzip(backup.GetItemOperationsList(), "backup item operations list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	podVolumeBackups, errs := encode.EncodeToJSONGzip(backup.PodVolumeBackups, "pod volume backups list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	csiSnapshotJSON, errs := encode.EncodeToJSONGzip(csiVolumeSnapshots, "csi volume snapshots list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	csiSnapshotContentsJSON, errs := encode.EncodeToJSONGzip(csiVolumeSnapshotContents, "csi volume snapshot contents list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}
	csiSnapshotClassesJSON, errs := encode.EncodeToJSONGzip(csiVolumesnapshotClasses, "csi volume snapshot classes list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	backupResourceList, errs := encode.EncodeToJSONGzip(backup.BackupResourceList(), "backup resources list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	backupResult, errs := encode.EncodeToJSONGzip(results, "backup results")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	if len(persistErrs) > 0 {
		// Don't upload the JSON files or backup tarball if encoding to json fails.
		backupJSON = nil
		backupContents = nil
		nativeVolumeSnapshots = nil
		backupItemOperations = nil
		backupResourceList = nil
		csiSnapshotJSON = nil
		csiSnapshotContentsJSON = nil
		csiSnapshotClassesJSON = nil
		backupResult = nil
	}

	backupInfo := persistence.BackupInfo{
		Name:                      backup.Name,
		Metadata:                  backupJSON,
		Contents:                  backupContents,
		Log:                       backupLog,
		BackupResults:             backupResult,
		PodVolumeBackups:          podVolumeBackups,
		VolumeSnapshots:           nativeVolumeSnapshots,
		BackupItemOperations:      backupItemOperations,
		BackupResourceList:        backupResourceList,
		CSIVolumeSnapshots:        csiSnapshotJSON,
		CSIVolumeSnapshotContents: csiSnapshotContentsJSON,
		CSIVolumeSnapshotClasses:  csiSnapshotClassesJSON,
	}
	if err := backupStore.PutBackup(backupInfo); err != nil {
		persistErrs = append(persistErrs, err)
	}

	return persistErrs
}

func closeAndRemoveFile(file *os.File, log logrus.FieldLogger) {
	if file == nil {
		log.Debug("Skipping removal of file due to nil file pointer")
		return
	}
	if err := file.Close(); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error closing file")
	}
	if err := os.Remove(file.Name()); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error removing file")
	}
}

// waitVolumeSnapshotReadyToUse is used to wait VolumeSnapshot turned to ReadyToUse.
// Waiting for VolumeSnapshot ReadyToUse to true is time consuming. Try to make the process parallel by
// using goroutine here instead of waiting in CSI plugin, because it's not easy to make BackupItemAction
// parallel by now. After BackupItemAction parallel is implemented, this logic should be moved to CSI plugin
// as https://github.com/vmware-tanzu/velero-plugin-for-csi/pull/100
func (b *backupReconciler) waitVolumeSnapshotReadyToUse(ctx context.Context,
	csiSnapshotTimeout time.Duration, backupName string) ([]snapshotv1api.VolumeSnapshot, error) {
	eg, _ := errgroup.WithContext(ctx)
	timeout := csiSnapshotTimeout
	interval := 5 * time.Second
	volumeSnapshots := make([]snapshotv1api.VolumeSnapshot, 0)

	if b.volumeSnapshotLister != nil {
		tmpVSs, err := b.volumeSnapshotLister.List(label.NewSelectorForBackup(backupName))
		if err != nil {
			b.logger.Error(err)
			return volumeSnapshots, err
		}
		for _, vs := range tmpVSs {
			volumeSnapshots = append(volumeSnapshots, *vs)
		}
	}

	vsChannel := make(chan snapshotv1api.VolumeSnapshot, len(volumeSnapshots))
	defer close(vsChannel)

	for index := range volumeSnapshots {
		volumeSnapshot := volumeSnapshots[index]
		eg.Go(func() error {
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				tmpVS, err := b.volumeSnapshotClient.SnapshotV1().VolumeSnapshots(volumeSnapshot.Namespace).Get(b.ctx, volumeSnapshot.Name, metav1.GetOptions{})
				if err != nil {
					return false, errors.Wrapf(err, fmt.Sprintf("failed to get volumesnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name))
				}
				if tmpVS.Status == nil || tmpVS.Status.BoundVolumeSnapshotContentName == nil || !boolptr.IsSetToTrue(tmpVS.Status.ReadyToUse) {
					b.logger.Infof("Waiting for CSI driver to reconcile volumesnapshot %s/%s. Retrying in %ds", volumeSnapshot.Namespace, volumeSnapshot.Name, interval/time.Second)
					return false, nil
				}

				b.logger.Debugf("VolumeSnapshot %s/%s turned into ReadyToUse.", volumeSnapshot.Namespace, volumeSnapshot.Name)
				// Put the ReadyToUse VolumeSnapshot element in the result channel.
				vsChannel <- *tmpVS
				return true, nil
			})
			if err == wait.ErrWaitTimeout {
				b.logger.Errorf("Timed out awaiting reconciliation of volumesnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			}
			return err
		})
	}

	err := eg.Wait()

	result := make([]snapshotv1api.VolumeSnapshot, 0)
	length := len(vsChannel)
	for index := 0; index < length; index++ {
		result = append(result, <-vsChannel)
	}

	return result, err
}

// deleteVolumeSnapshots delete VolumeSnapshot created during backup.
// This is used to avoid deleting namespace in cluster triggers the VolumeSnapshot deletion,
// which will cause snapshot deletion on cloud provider, then backup cannot restore the PV.
// If DeletionPolicy is Retain, just delete it. If DeletionPolicy is Delete, need to
// change DeletionPolicy to Retain before deleting VS, then change DeletionPolicy back to Delete.
func (b *backupReconciler) deleteVolumeSnapshots(volumeSnapshots []snapshotv1api.VolumeSnapshot,
	volumeSnapshotContents []snapshotv1api.VolumeSnapshotContent,
	logger logrus.FieldLogger, maxConcurrent int) {
	var wg sync.WaitGroup
	vscMap := make(map[string]snapshotv1api.VolumeSnapshotContent)
	for _, vsc := range volumeSnapshotContents {
		vscMap[vsc.Name] = vsc
	}

	ch := make(chan snapshotv1api.VolumeSnapshot, maxConcurrent)
	defer func() {
		if _, ok := <-ch; ok {
			close(ch)
		}
	}()

	wg.Add(maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		go func() {
			for {
				vs, ok := <-ch
				if !ok {
					wg.Done()
					return
				}
				b.deleteVolumeSnapshot(vs, vscMap, logger)
			}
		}()
	}

	for _, vs := range volumeSnapshots {
		ch <- vs
	}
	close(ch)

	wg.Wait()
}

// deleteVolumeSnapshot is called by deleteVolumeSnapshots and handles the single VolumeSnapshot
// instance.
func (b *backupReconciler) deleteVolumeSnapshot(vs snapshotv1api.VolumeSnapshot, vscMap map[string]snapshotv1api.VolumeSnapshotContent, logger logrus.FieldLogger) {
	var vsc snapshotv1api.VolumeSnapshotContent
	modifyVSCFlag := false
	if vs.Status != nil &&
		vs.Status.BoundVolumeSnapshotContentName != nil &&
		len(*vs.Status.BoundVolumeSnapshotContentName) > 0 {
		var found bool
		if vsc, found = vscMap[*vs.Status.BoundVolumeSnapshotContentName]; !found {
			logger.Errorf("Not find %s from the vscMap", *vs.Status.BoundVolumeSnapshotContentName)
			return
		}

		if vsc.Spec.DeletionPolicy == snapshotv1api.VolumeSnapshotContentDelete {
			modifyVSCFlag = true
		}
	} else {
		logger.Errorf("VolumeSnapshot %s/%s is not ready. This is not expected.", vs.Namespace, vs.Name)
	}

	// Change VolumeSnapshotContent's DeletionPolicy to Retain before deleting VolumeSnapshot,
	// because VolumeSnapshotContent will be deleted by deleting VolumeSnapshot, when
	// DeletionPolicy is set to Delete, but Velero needs VSC for cleaning snapshot on cloud
	// in backup deletion.
	if modifyVSCFlag {
		logger.Debugf("Patching VolumeSnapshotContent %s", vsc.Name)
		original := vsc.DeepCopy()
		vsc.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain
		if err := b.kbClient.Patch(context.Background(), &vsc, kbclient.MergeFrom(original)); err != nil {
			logger.Errorf("fail to modify VolumeSnapshotContent %s DeletionPolicy to Retain: %s", vsc.Name, err.Error())
			return
		}

		defer func() {
			logger.Debugf("Start to recreate VolumeSnapshotContent %s", vsc.Name)
			err := b.recreateVolumeSnapshotContent(vsc)
			if err != nil {
				logger.Errorf("fail to recreate VolumeSnapshotContent %s: %s", vsc.Name, err.Error())
			}
		}()
	}

	// Delete VolumeSnapshot from cluster
	logger.Debugf("Deleting VolumeSnapshot %s/%s", vs.Namespace, vs.Name)
	err := b.volumeSnapshotClient.SnapshotV1().VolumeSnapshots(vs.Namespace).Delete(context.TODO(), vs.Name, metav1.DeleteOptions{})
	if err != nil {
		logger.Errorf("fail to delete VolumeSnapshot %s/%s: %s", vs.Namespace, vs.Name, err.Error())
	}
}

// recreateVolumeSnapshotContent will delete then re-create VolumeSnapshotContent,
// because some parameter in VolumeSnapshotContent Spec is immutable, e.g. VolumeSnapshotRef
// and Source. Source is updated to let csi-controller thinks the VSC is statically provsisioned with VS.
// Set VolumeSnapshotRef's UID to nil will let the csi-controller finds out the related VS is gone, then
// VSC can be deleted.
func (b *backupReconciler) recreateVolumeSnapshotContent(vsc snapshotv1api.VolumeSnapshotContent) error {
	timeout := b.resourceTimeout
	interval := 1 * time.Second

	err := b.kbClient.Delete(context.TODO(), &vsc)
	if err != nil {
		return errors.Wrapf(err, "fail to delete VolumeSnapshotContent: %s", vsc.Name)
	}

	// Check VolumeSnapshotContents is already deleted, before re-creating it.
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		tmpVSC := &snapshotv1api.VolumeSnapshotContent{}
		err := b.kbClient.Get(context.TODO(), kbclient.ObjectKey{Name: vsc.Name}, tmpVSC)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, errors.Wrapf(err, fmt.Sprintf("failed to get VolumeSnapshotContent %s", vsc.Name))
		}
		return false, nil
	})
	if err != nil {
		return errors.Wrapf(err, "fail to retrieve VolumeSnapshotContent %s info", vsc.Name)
	}

	// Make the VolumeSnapshotContent static
	vsc.Spec.Source = snapshotv1api.VolumeSnapshotContentSource{
		SnapshotHandle: vsc.Status.SnapshotHandle,
	}
	// Set VolumeSnapshotRef to none exist one, because VolumeSnapshotContent
	// validation webhook will check whether name and namespace are nil.
	// external-snapshotter needs Source pointing to snapshot and VolumeSnapshot
	// reference's UID to nil to determine the VolumeSnapshotContent is deletable.
	vsc.Spec.VolumeSnapshotRef = v1.ObjectReference{
		APIVersion: snapshotv1api.SchemeGroupVersion.String(),
		Kind:       "VolumeSnapshot",
		Namespace:  "ns-" + string(vsc.UID),
		Name:       "name-" + string(vsc.UID),
	}
	// ResourceVersion shouldn't exist for new creation.
	vsc.ResourceVersion = ""
	err = b.kbClient.Create(context.TODO(), &vsc)
	if err != nil {
		return errors.Wrapf(err, "fail to create VolumeSnapshotContent %s", vsc.Name)
	}

	return nil
}

func oldAndNewFilterParametersUsedTogether(backupSpec velerov1api.BackupSpec) bool {
	haveOldResourceFilterParameters := len(backupSpec.IncludedResources) > 0 ||
		(len(backupSpec.ExcludedResources) > 0) ||
		(backupSpec.IncludeClusterResources != nil)
	haveNewResourceFilterParameters := len(backupSpec.IncludedClusterScopedResources) > 0 ||
		(len(backupSpec.ExcludedClusterScopedResources) > 0) ||
		(len(backupSpec.IncludedNamespaceScopedResources) > 0) ||
		(len(backupSpec.ExcludedNamespaceScopedResources) > 0)

	return haveOldResourceFilterParameters && haveNewResourceFilterParameters
}
