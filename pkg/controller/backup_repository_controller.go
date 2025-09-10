/*
Copyright 2018, 2019 the Velero contributors.

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
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/petar/GoLLRB/llrb"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/constant"
	"github.com/vmware-tanzu/velero/pkg/label"
	repoconfig "github.com/vmware-tanzu/velero/pkg/repository/config"
	"github.com/vmware-tanzu/velero/pkg/repository/maintenance"
	repomanager "github.com/vmware-tanzu/velero/pkg/repository/manager"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

const (
	repoSyncPeriod                      = 5 * time.Minute
	defaultMaintainFrequency            = 7 * 24 * time.Hour
	defaultMaintenanceStatusQueueLength = 3
)

type BackupRepoReconciler struct {
	client.Client
	namespace             string
	logger                logrus.FieldLogger
	clock                 clocks.WithTickerAndDelayedExecution
	maintenanceFrequency  time.Duration
	backupRepoConfig      string
	repositoryManager     repomanager.Manager
	repoMaintenanceConfig string
	logLevel              logrus.Level
	logFormat             *logging.FormatFlag

	// Startup validation fields
	startupValidationDone bool
	startupMutex          sync.Mutex
}

func NewBackupRepoReconciler(
	namespace string,
	logger logrus.FieldLogger,
	client client.Client,
	repositoryManager repomanager.Manager,
	maintenanceFrequency time.Duration,
	backupRepoConfig string,
	repoMaintenanceConfig string,
	logLevel logrus.Level,
	logFormat *logging.FormatFlag,
) *BackupRepoReconciler {
	c := &BackupRepoReconciler{
		Client:                client,
		namespace:             namespace,
		logger:                logger,
		clock:                 clocks.RealClock{},
		maintenanceFrequency:  maintenanceFrequency,
		backupRepoConfig:      backupRepoConfig,
		repositoryManager:     repositoryManager,
		repoMaintenanceConfig: repoMaintenanceConfig,
		logLevel:              logLevel,
		logFormat:             logFormat,
		startupValidationDone: false,
		startupMutex:          sync.Mutex{},
	}

	return c
}

func (r *BackupRepoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(
		r.logger.WithField("controller", constant.ControllerBackupRepo),
		mgr.GetClient(),
		&velerov1api.BackupRepositoryList{},
		repoSyncPeriod,
		kube.PeriodicalEnqueueSourceOption{},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupRepository{}, builder.WithPredicates(kube.SpecChangePredicate{})).
		WatchesRawSource(s).
		Watches(
			// mark BackupRepository as invalid when BSL is created, updated or deleted.
			// BSL may be recreated after deleting, so also include the create event
			&velerov1api.BackupStorageLocation{},
			kube.EnqueueRequestsFromMapUpdateFunc(r.invalidateBackupReposForBSL),
			builder.WithPredicates(
				// Combine three predicates together to guarantee
				// only BSL's Delete Event and Update Event can enqueue.
				// We don't care about BSL's Generic Event and Create Event,
				// because BSL's periodical enqueue triggers Generic Event,
				// and the BackupRepository controller restart will triggers BSL create event.
				kube.NewUpdateEventPredicate(
					r.needInvalidBackupRepo,
				),
				kube.NewGenericEventPredicate(
					func(client.Object) bool { return false },
				),
				kube.NewCreateEventPredicate(
					func(client.Object) bool { return false },
				),
			),
		).
		Complete(r)
}

func (r *BackupRepoReconciler) invalidateBackupReposForBSL(ctx context.Context, bslObj client.Object) []reconcile.Request {
	bsl := bslObj.(*velerov1api.BackupStorageLocation)

	list := &velerov1api.BackupRepositoryList{}
	options := &client.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			velerov1api.StorageLocationLabel: label.GetValidName(bsl.Name),
		}).AsSelector(),
	}
	if err := r.List(context.TODO(), list, options); err != nil {
		r.logger.WithField("BSL", bsl.Name).WithError(err).Error("unable to list BackupRepositories")
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for i := range list.Items {
		r.logger.WithField("BSL", bsl.Name).Infof("Invalidating Backup Repository %s", list.Items[i].Name)
		if err := r.patchBackupRepository(context.Background(), &list.Items[i], repoNotReady("re-establish on BSL change, create or delete")); err != nil {
			r.logger.WithField("BSL", bsl.Name).WithError(err).Errorf("fail to patch BackupRepository %s", list.Items[i].Name)
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: list.Items[i].Namespace, Name: list.Items[i].Name}})
	}

	return requests
}

// validateBackupRepositoriesOnStartup checks all BackupRepositories on controller startup
// and invalidates any that have mismatched BSL configuration
func (r *BackupRepoReconciler) validateBackupRepositoriesOnStartup(ctx context.Context) error {
	r.logger.Info("Starting validation of backup repositories on startup")

	// List all BackupRepositories
	repoList := &velerov1api.BackupRepositoryList{}
	if err := r.List(ctx, repoList); err != nil {
		return errors.Wrap(err, "failed to list backup repositories")
	}
	r.logger.Infof("Found %d repositories to validate", len(repoList.Items))

	// List all BSLs
	bslList := &velerov1api.BackupStorageLocationList{}
	if err := r.List(ctx, bslList); err != nil {
		return errors.Wrap(err, "failed to list backup storage locations")
	}
	r.logger.Infof("Found %d BSLs", len(bslList.Items))

	// Create map of BSL name to BSL for quick lookup
	bslMap := make(map[string]*velerov1api.BackupStorageLocation)
	for i := range bslList.Items {
		bslMap[bslList.Items[i].Name] = &bslList.Items[i]
	}

	invalidatedCount := 0
	for i := range repoList.Items {
		repo := &repoList.Items[i]
		r.logger.WithField("repo", repo.Name).Debugf("Checking repository with phase %s", repo.Status.Phase)

		// Skip if repository is new (not yet initialized)
		if repo.Status.Phase == "" || repo.Status.Phase == velerov1api.BackupRepositoryPhaseNew {
			r.logger.WithField("repo", repo.Name).Debug("Skipping new/uninitialized repository")
			continue
		}

		// Get the associated BSL
		bsl, exists := bslMap[repo.Spec.BackupStorageLocation]
		if !exists {
			r.logger.WithField("repo", repo.Name).Warn("BSL not found for repository, skipping validation")
			continue
		}

		// Check if BSL config has changed by comparing stored BSL info with current
		needsInvalidation := r.needInvalidBackupRepoOnStartup(repo, bsl)
		r.logger.WithFields(logrus.Fields{
			"repo":              repo.Name,
			"bsl":               bsl.Name,
			"needsInvalidation": needsInvalidation,
		}).Debug("Validation check completed")

		if needsInvalidation {
			r.logger.WithFields(logrus.Fields{
				"repo": repo.Name,
				"bsl":  bsl.Name,
			}).Info("Invalidating backup repository due to BSL configuration change detected on startup")

			if err := r.patchBackupRepository(ctx, repo, repoNotReady("BSL configuration changed while Velero was not running, re-establishing connection")); err != nil {
				r.logger.WithField("repo", repo.Name).WithError(err).Error("Failed to invalidate backup repository")
				continue
			}
			invalidatedCount++
		}
	}

	r.logger.WithField("invalidatedCount", invalidatedCount).Info("Completed backup repository validation on startup")
	return nil
}

// compareBSLConfigs compares two BSL configurations and returns true if they differ
// in fields that would require repository invalidation (bucket, prefix, CACert, or config)
func (r *BackupRepoReconciler) compareBSLConfigs(
	oldBucket, newBucket string,
	oldPrefix, newPrefix string,
	oldCACert, newCACert []byte,
	oldConfig, newConfig map[string]string,
	contextName string,
) bool {
	logger := r.logger.WithField("context", contextName)

	if oldBucket != newBucket {
		logger.WithFields(logrus.Fields{
			"old bucket": oldBucket,
			"new bucket": newBucket,
		}).Info("BSL's bucket has changed, invalid backup repositories")
		return true
	}

	if oldPrefix != newPrefix {
		logger.WithFields(logrus.Fields{
			"old prefix": oldPrefix,
			"new prefix": newPrefix,
		}).Info("BSL's prefix has changed, invalid backup repositories")
		return true
	}

	if !bytes.Equal(oldCACert, newCACert) {
		logger.Info("BSL's CACert has changed, invalid backup repositories")
		return true
	}

	if !reflect.DeepEqual(oldConfig, newConfig) {
		logger.Info("BSL's storage config has changed, invalid backup repositories")
		return true
	}

	return false
}

// needInvalidBackupRepoOnStartup checks if a repository needs invalidation by comparing
// its stored BSL information with the current BSL configuration
func (r *BackupRepoReconciler) needInvalidBackupRepoOnStartup(repo *velerov1api.BackupRepository, currentBSL *velerov1api.BackupStorageLocation) bool {
	// For restic repositories, check if the identifier would change
	if repo.Spec.RepositoryType == "" || repo.Spec.RepositoryType == velerov1api.BackupRepositoryTypeRestic {
		currentIdentifier, err := r.getIdentifierByBSL(currentBSL, repo)
		if err != nil {
			r.logger.WithField("repo", repo.Name).WithError(err).Warn("Failed to calculate current identifier")
			return true // Invalidate on error to be safe
		}

		if currentIdentifier != repo.Spec.ResticIdentifier {
			r.logger.WithFields(logrus.Fields{
				"repo":          repo.Name,
				"oldIdentifier": repo.Spec.ResticIdentifier,
				"newIdentifier": currentIdentifier,
			}).Info("Repository identifier has changed")
			return true
		}
	}

	// Check stored BSL config annotations against current BSL
	if repo.Annotations == nil {
		// No annotations means this repo was created before we started tracking BSL config
		// We should invalidate it to be safe
		r.logger.WithField("repo", repo.Name).Info("No annotations found, invalidating repository")
		return true
	}

	currentStorage := currentBSL.Spec.StorageType.ObjectStorage
	if currentStorage == nil {
		currentStorage = &velerov1api.ObjectStorageLocation{}
	}

	// Log the comparison details
	r.logger.WithFields(logrus.Fields{
		"repo":          repo.Name,
		"storedBucket":  repo.Annotations[velerov1api.BSLBucketAnnotation],
		"currentBucket": currentStorage.Bucket,
		"storedPrefix":  repo.Annotations[velerov1api.BSLPrefixAnnotation],
		"currentPrefix": currentStorage.Prefix,
	}).Debug("Comparing BSL configurations")

	// Parse stored CACert
	storedCACert := []byte(repo.Annotations[velerov1api.BSLCACertAnnotation])

	// Parse stored config from JSON
	var storedConfig map[string]string
	if configJSON := repo.Annotations[velerov1api.BSLConfigAnnotation]; configJSON != "" && configJSON != "{}" {
		if err := json.Unmarshal([]byte(configJSON), &storedConfig); err != nil {
			// If we can't parse the stored config, invalidate to be safe
			r.logger.WithField("repo", repo.Name).WithError(err).Warn("Failed to parse stored BSL config")
			return true
		}
	}

	// Use the shared comparison logic
	return r.compareBSLConfigs(
		repo.Annotations[velerov1api.BSLBucketAnnotation], currentStorage.Bucket,
		repo.Annotations[velerov1api.BSLPrefixAnnotation], currentStorage.Prefix,
		storedCACert, currentStorage.CACert,
		storedConfig, currentBSL.Spec.Config,
		repo.Name,
	)
}

// needInvalidBackupRepo returns true if the BSL's storage type, bucket, prefix, CACert, or config has changed
func (r *BackupRepoReconciler) needInvalidBackupRepo(oldObj client.Object, newObj client.Object) bool {
	oldBSL := oldObj.(*velerov1api.BackupStorageLocation)
	newBSL := newObj.(*velerov1api.BackupStorageLocation)

	oldStorage := oldBSL.Spec.StorageType.ObjectStorage
	newStorage := newBSL.Spec.StorageType.ObjectStorage

	if oldStorage == nil {
		oldStorage = &velerov1api.ObjectStorageLocation{}
	}

	if newStorage == nil {
		newStorage = &velerov1api.ObjectStorageLocation{}
	}

	// Use the shared comparison logic
	return r.compareBSLConfigs(
		oldStorage.Bucket, newStorage.Bucket,
		oldStorage.Prefix, newStorage.Prefix,
		oldStorage.CACert, newStorage.CACert,
		oldBSL.Spec.Config, newBSL.Spec.Config,
		newBSL.Name,
	)
}

func (r *BackupRepoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Perform one-time startup validation synchronously on first reconciliation
	r.startupMutex.Lock()
	if !r.startupValidationDone {
		r.startupValidationDone = true
		r.startupMutex.Unlock()

		// Run validation synchronously to ensure it completes before normal reconciliation
		r.logger.WithField("repository", req.String()).Info("Running startup validation for backup repositories on first reconciliation")
		if err := r.validateBackupRepositoriesOnStartup(ctx); err != nil {
			r.logger.WithError(err).Error("Failed to validate backup repositories on startup")
		}
		r.logger.Info("Startup validation completed")
	} else {
		r.startupMutex.Unlock()
	}

	log := r.logger.WithField("backupRepo", req.String())
	backupRepo := &velerov1api.BackupRepository{}
	if err := r.Get(ctx, req.NamespacedName, backupRepo); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf("backup repository %s in namespace %s is not found", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("error getting backup repository")
		return ctrl.Result{}, err
	}

	bsl, bslErr := r.getBSL(ctx, backupRepo)
	if bslErr != nil {
		log.WithError(bslErr).Error("Fail to get BSL for BackupRepository. Skip reconciling.")
		return ctrl.Result{}, nil
	}

	if backupRepo.Status.Phase == "" || backupRepo.Status.Phase == velerov1api.BackupRepositoryPhaseNew {
		if err := r.initializeRepo(ctx, backupRepo, bsl, log); err != nil {
			log.WithError(err).Error("error initialize repository")
			return ctrl.Result{}, errors.WithStack(err)
		}

		return ctrl.Result{}, nil
	}

	// If the repository is ready or not-ready, check it for stale locks, but if
	// this fails for any reason, it's non-critical so we still continue on to the
	// rest of the "process" logic.
	log.Debug("Checking repository for stale locks")
	if err := r.repositoryManager.UnlockRepo(backupRepo); err != nil {
		log.WithError(err).Error("Error checking repository for stale locks")
	}

	switch backupRepo.Status.Phase {
	case velerov1api.BackupRepositoryPhaseNotReady:
		ready, err := r.checkNotReadyRepo(ctx, backupRepo, bsl, log)
		if err != nil {
			return ctrl.Result{}, err
		} else if !ready {
			return ctrl.Result{}, nil
		}
		fallthrough
	case velerov1api.BackupRepositoryPhaseReady:
		if bsl.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
			log.Debugf("Skip running maintenance for BackupRepository, because its BSL is in the ReadOnly mode.")
			return ctrl.Result{}, nil
		}

		if err := r.recallMaintenance(ctx, backupRepo, log); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "error handling incomplete repo maintenance jobs")
		}

		if err := r.runMaintenanceIfDue(ctx, backupRepo, log); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "error check and run repo maintenance jobs")
		}

		// Get the configured number of maintenance jobs to keep from ConfigMap
		keepJobs, err := maintenance.GetKeepLatestMaintenanceJobs(ctx, r.Client, log, r.namespace, r.repoMaintenanceConfig, backupRepo)
		if err != nil {
			log.WithError(err).Warn("Failed to get keepLatestMaintenanceJobs from ConfigMap, using CLI parameter value")
		}

		if err := maintenance.DeleteOldJobs(r.Client, req.Name, keepJobs, log); err != nil {
			log.WithError(err).Warn("Failed to delete old maintenance jobs")
		}
	}

	return ctrl.Result{}, nil
}

func (r *BackupRepoReconciler) getBSL(ctx context.Context, req *velerov1api.BackupRepository) (*velerov1api.BackupStorageLocation, error) {
	loc := new(velerov1api.BackupStorageLocation)

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Spec.BackupStorageLocation,
	}, loc); err != nil {
		return nil, err
	}

	return loc, nil
}

func (r *BackupRepoReconciler) getIdentifierByBSL(bsl *velerov1api.BackupStorageLocation, req *velerov1api.BackupRepository) (string, error) {
	repoIdentifier, err := repoconfig.GetRepoIdentifier(bsl, req.Spec.VolumeNamespace)
	if err != nil {
		return "", errors.Wrapf(err, "error to get identifier for repo %s", req.Name)
	}

	return repoIdentifier, nil
}

func (r *BackupRepoReconciler) initializeRepo(ctx context.Context, req *velerov1api.BackupRepository, bsl *velerov1api.BackupStorageLocation, log logrus.FieldLogger) error {
	log.WithField("repoConfig", r.backupRepoConfig).Info("Initializing backup repository")

	var repoIdentifier string
	// Only get restic identifier for restic repositories
	if req.Spec.RepositoryType == "" || req.Spec.RepositoryType == velerov1api.BackupRepositoryTypeRestic {
		var err error
		repoIdentifier, err = r.getIdentifierByBSL(bsl, req)
		if err != nil {
			return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
				rr.Status.Message = err.Error()
				rr.Status.Phase = velerov1api.BackupRepositoryPhaseNotReady

				if rr.Spec.MaintenanceFrequency.Duration <= 0 {
					rr.Spec.MaintenanceFrequency = metav1.Duration{Duration: r.getRepositoryMaintenanceFrequency(req)}
				}
			})
		}
	}

	config, err := getBackupRepositoryConfig(ctx, r, r.backupRepoConfig, r.namespace, req.Name, req.Spec.RepositoryType, log)
	if err != nil {
		log.WithError(err).Warn("Failed to get repo config, repo config is ignored")
	} else if config != nil {
		log.Infof("Init repo with config %v", config)
	}

	// defaulting - if the patch fails, return an error so the item is returned to the queue
	if err := r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		// Only set ResticIdentifier for restic repositories
		if rr.Spec.RepositoryType == "" || rr.Spec.RepositoryType == velerov1api.BackupRepositoryTypeRestic {
			rr.Spec.ResticIdentifier = repoIdentifier
		}

		if rr.Spec.MaintenanceFrequency.Duration <= 0 {
			rr.Spec.MaintenanceFrequency = metav1.Duration{Duration: r.getRepositoryMaintenanceFrequency(req)}
		}

		rr.Spec.RepositoryConfig = config

		// Store BSL configuration in annotations for startup validation
		if rr.Annotations == nil {
			rr.Annotations = make(map[string]string)
		}

		// Store BSL config for future comparison
		if bsl.Spec.StorageType.ObjectStorage != nil {
			rr.Annotations[velerov1api.BSLBucketAnnotation] = bsl.Spec.StorageType.ObjectStorage.Bucket
			rr.Annotations[velerov1api.BSLPrefixAnnotation] = bsl.Spec.StorageType.ObjectStorage.Prefix
			if len(bsl.Spec.StorageType.ObjectStorage.CACert) > 0 {
				rr.Annotations[velerov1api.BSLCACertAnnotation] = string(bsl.Spec.StorageType.ObjectStorage.CACert)
			} else {
				delete(rr.Annotations, velerov1api.BSLCACertAnnotation)
			}
		}

		// Store BSL config as JSON
		if bsl.Spec.Config != nil {
			if data, err := json.Marshal(bsl.Spec.Config); err == nil {
				rr.Annotations[velerov1api.BSLConfigAnnotation] = string(data)
			}
		} else {
			rr.Annotations[velerov1api.BSLConfigAnnotation] = "{}"
		}
	}); err != nil {
		return err
	}

	if err := ensureRepo(req, r.repositoryManager); err != nil {
		return r.patchBackupRepository(ctx, req, repoNotReady(err.Error()))
	}

	return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		rr.Status.Phase = velerov1api.BackupRepositoryPhaseReady
		rr.Status.LastMaintenanceTime = &metav1.Time{Time: time.Now()}
	})
}

func (r *BackupRepoReconciler) getRepositoryMaintenanceFrequency(req *velerov1api.BackupRepository) time.Duration {
	if r.maintenanceFrequency > 0 {
		r.logger.WithField("frequency", r.maintenanceFrequency).Info("Set user defined maintenance frequency")
		return r.maintenanceFrequency
	}

	frequency, err := r.repositoryManager.DefaultMaintenanceFrequency(req)
	if err != nil || frequency <= 0 {
		r.logger.WithError(err).WithField("returned frequency", frequency).Warn("Failed to get maitanance frequency, use the default one")
		frequency = defaultMaintainFrequency
	} else {
		r.logger.WithField("frequency", frequency).Info("Set maintenance according to repository suggestion")
	}

	return frequency
}

// ensureRepo calls repo manager's PrepareRepo to ensure the repo is ready for use.
// An error is returned if the repository can't be connected to or initialized.
func ensureRepo(repo *velerov1api.BackupRepository, repoManager repomanager.Manager) error {
	return repoManager.PrepareRepo(repo)
}

func (r *BackupRepoReconciler) recallMaintenance(ctx context.Context, req *velerov1api.BackupRepository, log logrus.FieldLogger) error {
	history, err := maintenance.WaitAllJobsComplete(ctx, r.Client, req, defaultMaintenanceStatusQueueLength, log)
	if err != nil {
		return errors.Wrapf(err, "error waiting incomplete repo maintenance job for repo %s", req.Name)
	}

	consolidated := consolidateHistory(history, req.Status.RecentMaintenance)
	if consolidated == nil {
		return nil
	}

	lastMaintenanceTime := getLastMaintenanceTimeFromHistory(consolidated)

	log.Warn("Updating backup repository because of unrecorded histories")

	return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		if lastMaintenanceTime != nil && (rr.Status.LastMaintenanceTime == nil || lastMaintenanceTime.After(rr.Status.LastMaintenanceTime.Time)) {
			if rr.Status.LastMaintenanceTime != nil {
				log.Warnf("Updating backup repository last maintenance time (%v) from history (%v)", rr.Status.LastMaintenanceTime.Time, lastMaintenanceTime.Time)
			} else {
				log.Warnf("Setting backup repository last maintenance time from history (%v)", lastMaintenanceTime.Time)
			}
			rr.Status.LastMaintenanceTime = lastMaintenanceTime
		}

		rr.Status.RecentMaintenance = consolidated
	})
}

type maintenanceStatusWrapper struct {
	status *velerov1api.BackupRepositoryMaintenanceStatus
}

func (w maintenanceStatusWrapper) Less(other llrb.Item) bool {
	return w.status.StartTimestamp.Before(other.(maintenanceStatusWrapper).status.StartTimestamp)
}

func consolidateHistory(coming, cur []velerov1api.BackupRepositoryMaintenanceStatus) []velerov1api.BackupRepositoryMaintenanceStatus {
	if len(coming) == 0 {
		return nil
	}

	if slices.EqualFunc(cur, coming, func(a, b velerov1api.BackupRepositoryMaintenanceStatus) bool {
		return a.StartTimestamp.Equal(b.StartTimestamp)
	}) {
		return nil
	}

	consolidator := llrb.New()
	for i := range cur {
		consolidator.ReplaceOrInsert(maintenanceStatusWrapper{&cur[i]})
	}

	for i := range coming {
		consolidator.ReplaceOrInsert(maintenanceStatusWrapper{&coming[i]})
	}

	truncated := []velerov1api.BackupRepositoryMaintenanceStatus{}
	for consolidator.Len() > 0 {
		if len(truncated) == defaultMaintenanceStatusQueueLength {
			break
		}

		item := consolidator.DeleteMax()
		truncated = append(truncated, *item.(maintenanceStatusWrapper).status)
	}

	slices.Reverse(truncated)

	if slices.EqualFunc(cur, truncated, func(a, b velerov1api.BackupRepositoryMaintenanceStatus) bool {
		return a.StartTimestamp.Equal(b.StartTimestamp)
	}) {
		return nil
	}

	return truncated
}

func getLastMaintenanceTimeFromHistory(history []velerov1api.BackupRepositoryMaintenanceStatus) *metav1.Time {
	time := history[0].CompleteTimestamp

	for i := range history {
		if history[i].CompleteTimestamp == nil {
			continue
		}

		if time == nil || time.Before(history[i].CompleteTimestamp) {
			time = history[i].CompleteTimestamp
		}
	}

	return time
}

var funcStartMaintenanceJob = maintenance.StartNewJob
var funcWaitMaintenanceJobComplete = maintenance.WaitJobComplete

func (r *BackupRepoReconciler) runMaintenanceIfDue(ctx context.Context, req *velerov1api.BackupRepository, log logrus.FieldLogger) error {
	startTime := r.clock.Now()

	if !dueForMaintenance(req, startTime) {
		log.Debug("not due for maintenance")
		return nil
	}

	log.Info("Running maintenance on backup repository")

	job, err := funcStartMaintenanceJob(r.Client, ctx, req, r.repoMaintenanceConfig, r.logLevel, r.logFormat, log)
	if err != nil {
		log.WithError(err).Warn("Starting repo maintenance failed")
		return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
			updateRepoMaintenanceHistory(rr, velerov1api.BackupRepositoryMaintenanceFailed, &metav1.Time{Time: startTime}, nil, fmt.Sprintf("Failed to start maintenance job, err: %v", err))
		})
	}

	// when WaitMaintenanceJobComplete fails, the maintenance result will be left aside temporarily
	// If the maintenenance still completes later, recallMaintenance recalls the left once and update LastMaintenanceTime and history
	status, err := funcWaitMaintenanceJobComplete(r.Client, ctx, job, r.namespace, log)
	if err != nil {
		return errors.Wrapf(err, "error waiting repo maintenance completion status")
	}

	if status.Result == velerov1api.BackupRepositoryMaintenanceFailed {
		log.WithError(err).Warn("Pruning repository failed")
		return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
			updateRepoMaintenanceHistory(rr, velerov1api.BackupRepositoryMaintenanceFailed, status.StartTimestamp, status.CompleteTimestamp, status.Message)
		})
	}

	return r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		rr.Status.LastMaintenanceTime = &metav1.Time{Time: status.CompleteTimestamp.Time}
		updateRepoMaintenanceHistory(rr, velerov1api.BackupRepositoryMaintenanceSucceeded, status.StartTimestamp, status.CompleteTimestamp, status.Message)
	})
}

func updateRepoMaintenanceHistory(repo *velerov1api.BackupRepository, result velerov1api.BackupRepositoryMaintenanceResult, startTime, completionTime *metav1.Time, message string) {
	latest := velerov1api.BackupRepositoryMaintenanceStatus{
		Result:            result,
		StartTimestamp:    startTime,
		CompleteTimestamp: completionTime,
		Message:           message,
	}

	startingPos := 0
	if len(repo.Status.RecentMaintenance) >= defaultMaintenanceStatusQueueLength {
		startingPos = len(repo.Status.RecentMaintenance) - defaultMaintenanceStatusQueueLength + 1
	}

	repo.Status.RecentMaintenance = append(repo.Status.RecentMaintenance[startingPos:], latest)
}

func dueForMaintenance(req *velerov1api.BackupRepository, now time.Time) bool {
	return req.Status.LastMaintenanceTime == nil || req.Status.LastMaintenanceTime.Add(req.Spec.MaintenanceFrequency.Duration).Before(now)
}

func (r *BackupRepoReconciler) checkNotReadyRepo(ctx context.Context, req *velerov1api.BackupRepository, bsl *velerov1api.BackupStorageLocation, log logrus.FieldLogger) (bool, error) {
	log.Info("Checking backup repository for readiness")

	// Only check and update restic identifier for restic repositories
	if req.Spec.RepositoryType == "" || req.Spec.RepositoryType == velerov1api.BackupRepositoryTypeRestic {
		repoIdentifier, err := r.getIdentifierByBSL(bsl, req)
		if err != nil {
			return false, r.patchBackupRepository(ctx, req, repoNotReady(err.Error()))
		}

		if repoIdentifier != req.Spec.ResticIdentifier {
			if err := r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
				rr.Spec.ResticIdentifier = repoIdentifier
			}); err != nil {
				return false, err
			}
		}
	}

	// we need to ensure it (first check, if check fails, attempt to init)
	// because we don't know if it's been successfully initialized yet.
	if err := ensureRepo(req, r.repositoryManager); err != nil {
		return false, r.patchBackupRepository(ctx, req, repoNotReady(err.Error()))
	}

	// Update repository status to ready and update BSL annotations
	err := r.patchBackupRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		repoReady()(rr)

		// Update BSL configuration in annotations for startup validation
		if rr.Annotations == nil {
			rr.Annotations = make(map[string]string)
		}

		// Store BSL config for future comparison
		if bsl.Spec.StorageType.ObjectStorage != nil {
			rr.Annotations[velerov1api.BSLBucketAnnotation] = bsl.Spec.StorageType.ObjectStorage.Bucket
			rr.Annotations[velerov1api.BSLPrefixAnnotation] = bsl.Spec.StorageType.ObjectStorage.Prefix
			if len(bsl.Spec.StorageType.ObjectStorage.CACert) > 0 {
				rr.Annotations[velerov1api.BSLCACertAnnotation] = string(bsl.Spec.StorageType.ObjectStorage.CACert)
			} else {
				delete(rr.Annotations, velerov1api.BSLCACertAnnotation)
			}
		}

		// Store BSL config as JSON
		if bsl.Spec.Config != nil {
			if data, err := json.Marshal(bsl.Spec.Config); err == nil {
				rr.Annotations[velerov1api.BSLConfigAnnotation] = string(data)
			}
		} else {
			rr.Annotations[velerov1api.BSLConfigAnnotation] = "{}"
		}
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func repoNotReady(msg string) func(*velerov1api.BackupRepository) {
	return func(r *velerov1api.BackupRepository) {
		r.Status.Phase = velerov1api.BackupRepositoryPhaseNotReady
		r.Status.Message = msg
	}
}

func repoReady() func(*velerov1api.BackupRepository) {
	return func(r *velerov1api.BackupRepository) {
		r.Status.Phase = velerov1api.BackupRepositoryPhaseReady
		r.Status.Message = ""
	}
}

// patchBackupRepository mutates req with the provided mutate function, and patches it
// through the Kube API. After executing this function, req will be updated with both
// the mutation and the results of the Patch() API call.
func (r *BackupRepoReconciler) patchBackupRepository(ctx context.Context, req *velerov1api.BackupRepository, mutate func(*velerov1api.BackupRepository)) error {
	original := req.DeepCopy()
	mutate(req)
	if err := r.Patch(ctx, req, client.MergeFrom(original)); err != nil {
		return errors.Wrap(err, "error patching BackupRepository")
	}
	return nil
}

func getBackupRepositoryConfig(ctx context.Context, ctrlClient client.Client, configName, namespace, repoName, repoType string, log logrus.FieldLogger) (map[string]string, error) {
	if configName == "" {
		return nil, nil
	}

	loc := &corev1api.ConfigMap{}
	if err := ctrlClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      configName,
	}, loc); err != nil {
		return nil, errors.Wrapf(err, "error getting configMap %s", configName)
	}

	jsonData, found := loc.Data[repoType]
	if !found {
		log.Info("No data for repo type %s in config map %s", repoType, configName)
		return nil, nil
	}

	var unmarshalled map[string]any
	if err := json.Unmarshal([]byte(jsonData), &unmarshalled); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling config data from %s for repo %s, repo type %s", configName, repoName, repoType)
	}

	result := map[string]string{}
	for k, v := range unmarshalled {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result, nil
}
