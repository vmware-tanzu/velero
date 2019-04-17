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
	"encoding/json"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions/velero/v1"
	listers "github.com/heptio/velero/pkg/generated/listers/velero/v1"
	"github.com/heptio/velero/pkg/restic"
)

type resticRepositoryController struct {
	*genericController

	resticRepositoryClient velerov1client.ResticRepositoriesGetter
	resticRepositoryLister listers.ResticRepositoryLister
	backupLocationLister   listers.BackupStorageLocationLister
	repositoryManager      restic.RepositoryManager

	clock clock.Clock
}

// NewResticRepositoryController creates a new restic repository controller.
func NewResticRepositoryController(
	logger logrus.FieldLogger,
	resticRepositoryInformer informers.ResticRepositoryInformer,
	resticRepositoryClient velerov1client.ResticRepositoriesGetter,
	backupLocationInformer informers.BackupStorageLocationInformer,
	repositoryManager restic.RepositoryManager,
) Interface {
	c := &resticRepositoryController{
		genericController:      newGenericController("restic-repository", logger),
		resticRepositoryClient: resticRepositoryClient,
		resticRepositoryLister: resticRepositoryInformer.Lister(),
		backupLocationLister:   backupLocationInformer.Lister(),
		repositoryManager:      repositoryManager,
		clock:                  &clock.RealClock{},
	}

	c.syncHandler = c.processQueueItem
	c.cacheSyncWaiters = append(c.cacheSyncWaiters, resticRepositoryInformer.Informer().HasSynced, backupLocationInformer.Informer().HasSynced)

	resticRepositoryInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueue,
		},
	)

	c.resyncPeriod = 5 * time.Minute
	c.resyncFunc = c.enqueueAllRepositories

	return c
}

// enqueueAllRepositories lists all restic repositories from cache and enqueues all
// of them so we can check each one for maintenance.
func (c *resticRepositoryController) enqueueAllRepositories() {
	c.logger.Debug("resticRepositoryController.enqueueAllRepositories")

	repos, err := c.resticRepositoryLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("error listing restic repositories")
		return
	}

	for _, repo := range repos {
		c.enqueue(repo)
	}
}

func (c *resticRepositoryController) processQueueItem(key string) error {
	log := c.logger.WithField("key", key)
	log.Debug("Running processQueueItem")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("error splitting queue key")
		return nil
	}

	log = c.logger.WithField("namespace", ns).WithField("name", name)

	req, err := c.resticRepositoryLister.ResticRepositories(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find ResticRepository")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting ResticRepository")
	}

	// Don't mutate the shared cache
	reqCopy := req.DeepCopy()

	switch req.Status.Phase {
	case "", v1.ResticRepositoryPhaseNew:
		return c.initializeRepo(reqCopy, log)
	case v1.ResticRepositoryPhaseReady:
		return c.runMaintenanceIfDue(reqCopy, log)
	case v1.ResticRepositoryPhaseNotReady:
		return c.checkNotReadyRepo(reqCopy, log)
	}

	return nil
}

func (c *resticRepositoryController) initializeRepo(req *v1.ResticRepository, log logrus.FieldLogger) error {
	log.Info("Initializing restic repository")

	// confirm the repo's BackupStorageLocation is valid
	loc, err := c.backupLocationLister.BackupStorageLocations(req.Namespace).Get(req.Spec.BackupStorageLocation)
	if err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	// defaulting - if the patch fails, return an error so the item is returned to the queue
	if err := c.patchResticRepository(req, func(r *v1.ResticRepository) {
		r.Spec.ResticIdentifier = restic.GetRepoIdentifier(loc, r.Spec.VolumeNamespace)

		if r.Spec.MaintenanceFrequency.Duration <= 0 {
			r.Spec.MaintenanceFrequency = metav1.Duration{Duration: restic.DefaultMaintenanceFrequency}
		}
	}); err != nil {
		return err
	}

	if err := ensureRepo(req, c.repositoryManager); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	return c.patchResticRepository(req, func(req *v1.ResticRepository) {
		req.Status.Phase = v1.ResticRepositoryPhaseReady
		req.Status.LastMaintenanceTime = metav1.Time{Time: time.Now()}
	})
}

// ensureRepo first tries to connect to the repo, and returns if it succeeds. If it fails,
// it attempts to init the repo, and returns the result.
func ensureRepo(repo *v1.ResticRepository, repoManager restic.RepositoryManager) error {
	if repoManager.ConnectToRepo(repo) == nil {
		return nil
	}

	return repoManager.InitRepo(repo)
}

func (c *resticRepositoryController) runMaintenanceIfDue(req *v1.ResticRepository, log logrus.FieldLogger) error {
	log.Debug("resticRepositoryController.runMaintenanceIfDue")

	now := c.clock.Now()

	if !dueForMaintenance(req, now) {
		log.Debug("not due for maintenance")
		return nil
	}

	log.Info("Running maintenance on restic repository")

	log.Debug("Checking repo before prune")
	if err := c.repositoryManager.CheckRepo(req); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	// prune failures should be displayed in the `.status.message` field but
	// should not cause the repo to move to `NotReady`.
	log.Debug("Pruning repo")
	if err := c.repositoryManager.PruneRepo(req); err != nil {
		log.WithError(err).Warn("error pruning repository")
		if patchErr := c.patchResticRepository(req, func(r *v1.ResticRepository) {
			r.Status.Message = err.Error()
		}); patchErr != nil {
			return patchErr
		}
	}

	log.Debug("Checking repo after prune")
	if err := c.repositoryManager.CheckRepo(req); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	return c.patchResticRepository(req, func(req *v1.ResticRepository) {
		req.Status.LastMaintenanceTime = metav1.Time{Time: now}
	})
}

func dueForMaintenance(req *v1.ResticRepository, now time.Time) bool {
	return req.Status.LastMaintenanceTime.Add(req.Spec.MaintenanceFrequency.Duration).Before(now)
}

func (c *resticRepositoryController) checkNotReadyRepo(req *v1.ResticRepository, log logrus.FieldLogger) error {
	log.Info("Checking restic repository for readiness")

	// we need to ensure it (first check, if check fails, attempt to init)
	// because we don't know if it's been successfully initialized yet.
	if err := ensureRepo(req, c.repositoryManager); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	return c.patchResticRepository(req, repoReady())
}

func repoNotReady(msg string) func(*v1.ResticRepository) {
	return func(r *v1.ResticRepository) {
		r.Status.Phase = v1.ResticRepositoryPhaseNotReady
		r.Status.Message = msg
	}
}

func repoReady() func(*v1.ResticRepository) {
	return func(r *v1.ResticRepository) {
		r.Status.Phase = v1.ResticRepositoryPhaseReady
		r.Status.Message = ""
	}
}

// patchResticRepository mutates req with the provided mutate function, and patches it
// through the Kube API. After executing this function, req will be updated with both
// the mutation and the results of the Patch() API call.
func (c *resticRepositoryController) patchResticRepository(req *v1.ResticRepository, mutate func(*v1.ResticRepository)) error {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "error marshalling original ResticRepository")
	}

	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "error marshalling updated ResticRepository")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return errors.Wrap(err, "error creating json merge patch for ResticRepository")
	}

	// empty patch: don't apply
	if string(patchBytes) == "{}" {
		return nil
	}

	// patch, and if successful, update req
	var patched *v1.ResticRepository
	if patched, err = c.resticRepositoryClient.ResticRepositories(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes); err != nil {
		return errors.Wrap(err, "error patching ResticRepository")
	}
	req = patched

	return nil
}
