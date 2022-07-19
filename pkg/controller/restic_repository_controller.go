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
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	repoSyncPeriod = 5 * time.Minute
)

type ResticRepoReconciler struct {
	client.Client
	namespace                   string
	logger                      logrus.FieldLogger
	clock                       clock.Clock
	defaultMaintenanceFrequency time.Duration
	repositoryManager           restic.RepositoryManager
}

func NewResticRepoReconciler(namespace string, logger logrus.FieldLogger, client client.Client,
	defaultMaintenanceFrequency time.Duration, repositoryManager restic.RepositoryManager) *ResticRepoReconciler {
	c := &ResticRepoReconciler{
		client,
		namespace,
		logger,
		clock.RealClock{},
		defaultMaintenanceFrequency,
		repositoryManager,
	}

	if c.defaultMaintenanceFrequency <= 0 {
		logger.Infof("Invalid default restic maintenance frequency, setting to %v", restic.DefaultMaintenanceFrequency)
		c.defaultMaintenanceFrequency = restic.DefaultMaintenanceFrequency
	}

	return c
}

func (r *ResticRepoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(r.logger, mgr.GetClient(), &velerov1api.BackupRepositoryList{}, repoSyncPeriod)
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupRepository{}).
		Watches(s, nil).
		Complete(r)
}

func (r *ResticRepoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithField("resticRepo", req.String())
	resticRepo := &velerov1api.BackupRepository{}
	if err := r.Get(ctx, req.NamespacedName, resticRepo); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf("restic repository %s in namespace %s is not found", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("error getting restic repository")
		return ctrl.Result{}, err
	}

	if resticRepo.Status.Phase == "" || resticRepo.Status.Phase == velerov1api.BackupRepositoryPhaseNew {
		if err := r.initializeRepo(ctx, resticRepo, log); err != nil {
			log.WithError(err).Error("error initialize repository")
			return ctrl.Result{}, errors.WithStack(err)
		}

		return ctrl.Result{}, nil
	}

	// If the repository is ready or not-ready, check it for stale locks, but if
	// this fails for any reason, it's non-critical so we still continue on to the
	// rest of the "process" logic.
	log.Debug("Checking repository for stale locks")
	if err := r.repositoryManager.UnlockRepo(resticRepo); err != nil {
		log.WithError(err).Error("Error checking repository for stale locks")
	}

	switch resticRepo.Status.Phase {
	case velerov1api.BackupRepositoryPhaseReady:
		return ctrl.Result{}, r.runMaintenanceIfDue(ctx, resticRepo, log)
	case velerov1api.BackupRepositoryPhaseNotReady:
		return ctrl.Result{}, r.checkNotReadyRepo(ctx, resticRepo, log)
	}

	return ctrl.Result{}, nil
}

func (r *ResticRepoReconciler) initializeRepo(ctx context.Context, req *velerov1api.BackupRepository, log logrus.FieldLogger) error {
	log.Info("Initializing restic repository")

	// confirm the repo's BackupStorageLocation is valid
	loc := &velerov1api.BackupStorageLocation{}

	if err := r.Get(context.Background(), client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Spec.BackupStorageLocation,
	}, loc); err != nil {
		return r.patchResticRepository(ctx, req, repoNotReady(err.Error()))
	}

	repoIdentifier, err := restic.GetRepoIdentifier(loc, req.Spec.VolumeNamespace)
	if err != nil {
		return r.patchResticRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
			rr.Status.Message = err.Error()
			rr.Status.Phase = velerov1api.BackupRepositoryPhaseNotReady

			if rr.Spec.MaintenanceFrequency.Duration <= 0 {
				rr.Spec.MaintenanceFrequency = metav1.Duration{Duration: r.defaultMaintenanceFrequency}
			}
		})
	}

	// defaulting - if the patch fails, return an error so the item is returned to the queue
	if err := r.patchResticRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		rr.Spec.ResticIdentifier = repoIdentifier

		if rr.Spec.MaintenanceFrequency.Duration <= 0 {
			rr.Spec.MaintenanceFrequency = metav1.Duration{Duration: r.defaultMaintenanceFrequency}
		}
	}); err != nil {
		return err
	}

	if err := ensureRepo(req, r.repositoryManager); err != nil {
		return r.patchResticRepository(ctx, req, repoNotReady(err.Error()))
	}

	return r.patchResticRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		rr.Status.Phase = velerov1api.BackupRepositoryPhaseReady
		rr.Status.LastMaintenanceTime = &metav1.Time{Time: time.Now()}
	})
}

// ensureRepo checks to see if a repository exists, and attempts to initialize it if
// it does not exist. An error is returned if the repository can't be connected to
// or initialized.
func ensureRepo(repo *velerov1api.BackupRepository, repoManager restic.RepositoryManager) error {
	if err := repoManager.ConnectToRepo(repo); err != nil {
		// If the repository has not yet been initialized, the error message will always include
		// the following string. This is the only scenario where we should try to initialize it.
		// Other errors (e.g. "already locked") should be returned as-is since the repository
		// does already exist, but it can't be connected to.
		if strings.Contains(err.Error(), "Is there a repository at the following location?") {
			return repoManager.InitRepo(repo)
		}

		return err
	}

	return nil
}

func (r *ResticRepoReconciler) runMaintenanceIfDue(ctx context.Context, req *velerov1api.BackupRepository, log logrus.FieldLogger) error {
	log.Debug("resticRepositoryController.runMaintenanceIfDue")

	now := r.clock.Now()

	if !dueForMaintenance(req, now) {
		log.Debug("not due for maintenance")
		return nil
	}

	log.Info("Running maintenance on restic repository")

	// prune failures should be displayed in the `.status.message` field but
	// should not cause the repo to move to `NotReady`.
	log.Debug("Pruning repo")
	if err := r.repositoryManager.PruneRepo(req); err != nil {
		log.WithError(err).Warn("error pruning repository")
		return r.patchResticRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
			rr.Status.Message = err.Error()
		})
	}

	return r.patchResticRepository(ctx, req, func(rr *velerov1api.BackupRepository) {
		rr.Status.LastMaintenanceTime = &metav1.Time{Time: now}
	})
}

func dueForMaintenance(req *velerov1api.BackupRepository, now time.Time) bool {
	return req.Status.LastMaintenanceTime == nil || req.Status.LastMaintenanceTime.Add(req.Spec.MaintenanceFrequency.Duration).Before(now)
}

func (r *ResticRepoReconciler) checkNotReadyRepo(ctx context.Context, req *velerov1api.BackupRepository, log logrus.FieldLogger) error {
	// no identifier: can't possibly be ready, so just return
	if req.Spec.ResticIdentifier == "" {
		return nil
	}

	log.Info("Checking restic repository for readiness")

	// we need to ensure it (first check, if check fails, attempt to init)
	// because we don't know if it's been successfully initialized yet.
	if err := ensureRepo(req, r.repositoryManager); err != nil {
		return r.patchResticRepository(ctx, req, repoNotReady(err.Error()))
	}
	return r.patchResticRepository(ctx, req, repoReady())
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

// patchResticRepository mutates req with the provided mutate function, and patches it
// through the Kube API. After executing this function, req will be updated with both
// the mutation and the results of the Patch() API call.
func (r *ResticRepoReconciler) patchResticRepository(ctx context.Context, req *velerov1api.BackupRepository, mutate func(*velerov1api.BackupRepository)) error {
	original := req.DeepCopy()
	mutate(req)
	if err := r.Patch(ctx, req, client.MergeFrom(original)); err != nil {
		return errors.Wrap(err, "error patching ResticRepository")
	}
	return nil
}
