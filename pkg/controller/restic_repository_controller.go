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
	"encoding/json"
	"strings"
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

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/restic"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type resticRepositoryController struct {
	*genericController

	resticRepositoryClient      velerov1client.ResticRepositoriesGetter
	resticRepositoryLister      velerov1listers.ResticRepositoryLister
	kbClient                    client.Client
	repositoryManager           restic.RepositoryManager
	defaultMaintenanceFrequency time.Duration

	clock clock.Clock
}

// NewResticRepositoryController creates a new restic repository controller.
func NewResticRepositoryController(
	logger logrus.FieldLogger,
	resticRepositoryInformer velerov1informers.ResticRepositoryInformer,
	resticRepositoryClient velerov1client.ResticRepositoriesGetter,
	kbClient client.Client,
	repositoryManager restic.RepositoryManager,
	defaultMaintenanceFrequency time.Duration,
) Interface {
	c := &resticRepositoryController{
		genericController:           newGenericController(ResticRepo, logger),
		resticRepositoryClient:      resticRepositoryClient,
		resticRepositoryLister:      resticRepositoryInformer.Lister(),
		kbClient:                    kbClient,
		repositoryManager:           repositoryManager,
		defaultMaintenanceFrequency: defaultMaintenanceFrequency,

		clock: &clock.RealClock{},
	}

	if c.defaultMaintenanceFrequency <= 0 {
		logger.Infof("Invalid default restic maintenance frequency, setting to %v", restic.DefaultMaintenanceFrequency)
		c.defaultMaintenanceFrequency = restic.DefaultMaintenanceFrequency
	}

	c.syncHandler = c.processQueueItem

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

	if req.Status.Phase == "" || req.Status.Phase == velerov1api.ResticRepositoryPhaseNew {
		return c.initializeRepo(reqCopy, log)
	}

	// If the repository is ready or not-ready, check it for stale locks, but if
	// this fails for any reason, it's non-critical so we still continue on to the
	// rest of the "process" logic.
	log.Debug("Checking repository for stale locks")
	if err := c.repositoryManager.UnlockRepo(reqCopy); err != nil {
		log.WithError(err).Error("Error checking repository for stale locks")
	}

	switch req.Status.Phase {
	case velerov1api.ResticRepositoryPhaseReady:
		return c.runMaintenanceIfDue(reqCopy, log)
	case velerov1api.ResticRepositoryPhaseNotReady:
		return c.checkNotReadyRepo(reqCopy, log)
	}

	return nil
}

func (c *resticRepositoryController) initializeRepo(req *velerov1api.ResticRepository, log logrus.FieldLogger) error {
	log.Info("Initializing restic repository")

	// confirm the repo's BackupStorageLocation is valid
	loc := &velerov1api.BackupStorageLocation{}
	if err := c.kbClient.Get(context.Background(), client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Spec.BackupStorageLocation,
	}, loc); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	repoIdentifier, err := restic.GetRepoIdentifier(loc, req.Spec.VolumeNamespace)
	if err != nil {
		return c.patchResticRepository(req, func(r *velerov1api.ResticRepository) {
			r.Status.Message = err.Error()
			r.Status.Phase = velerov1api.ResticRepositoryPhaseNotReady

			if r.Spec.MaintenanceFrequency.Duration <= 0 {
				r.Spec.MaintenanceFrequency = metav1.Duration{Duration: c.defaultMaintenanceFrequency}
			}
		})
	}

	// defaulting - if the patch fails, return an error so the item is returned to the queue
	if err := c.patchResticRepository(req, func(r *velerov1api.ResticRepository) {
		r.Spec.ResticIdentifier = repoIdentifier

		if r.Spec.MaintenanceFrequency.Duration <= 0 {
			r.Spec.MaintenanceFrequency = metav1.Duration{Duration: c.defaultMaintenanceFrequency}
		}
	}); err != nil {
		return err
	}

	if err := ensureRepo(req, c.repositoryManager); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	return c.patchResticRepository(req, func(req *velerov1api.ResticRepository) {
		req.Status.Phase = velerov1api.ResticRepositoryPhaseReady
		req.Status.LastMaintenanceTime = &metav1.Time{Time: time.Now()}
	})
}

// ensureRepo checks to see if a repository exists, and attempts to initialize it if
// it does not exist. An error is returned if the repository can't be connected to
// or initialized.
func ensureRepo(repo *velerov1api.ResticRepository, repoManager restic.RepositoryManager) error {
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

func (c *resticRepositoryController) runMaintenanceIfDue(req *velerov1api.ResticRepository, log logrus.FieldLogger) error {
	log.Debug("resticRepositoryController.runMaintenanceIfDue")

	now := c.clock.Now()

	if !dueForMaintenance(req, now) {
		log.Debug("not due for maintenance")
		return nil
	}

	log.Info("Running maintenance on restic repository")

	// prune failures should be displayed in the `.status.message` field but
	// should not cause the repo to move to `NotReady`.
	log.Debug("Pruning repo")
	if err := c.repositoryManager.PruneRepo(req); err != nil {
		log.WithError(err).Warn("error pruning repository")
		if patchErr := c.patchResticRepository(req, func(r *velerov1api.ResticRepository) {
			r.Status.Message = err.Error()
		}); patchErr != nil {
			return patchErr
		}
	}

	return c.patchResticRepository(req, func(req *velerov1api.ResticRepository) {
		req.Status.LastMaintenanceTime = &metav1.Time{Time: now}
	})
}

func dueForMaintenance(req *velerov1api.ResticRepository, now time.Time) bool {
	return req.Status.LastMaintenanceTime == nil || req.Status.LastMaintenanceTime.Add(req.Spec.MaintenanceFrequency.Duration).Before(now)
}

func (c *resticRepositoryController) checkNotReadyRepo(req *velerov1api.ResticRepository, log logrus.FieldLogger) error {
	// no identifier: can't possibly be ready, so just return
	if req.Spec.ResticIdentifier == "" {
		return nil
	}

	log.Info("Checking restic repository for readiness")

	// we need to ensure it (first check, if check fails, attempt to init)
	// because we don't know if it's been successfully initialized yet.
	if err := ensureRepo(req, c.repositoryManager); err != nil {
		return c.patchResticRepository(req, repoNotReady(err.Error()))
	}

	return c.patchResticRepository(req, repoReady())
}

func repoNotReady(msg string) func(*velerov1api.ResticRepository) {
	return func(r *velerov1api.ResticRepository) {
		r.Status.Phase = velerov1api.ResticRepositoryPhaseNotReady
		r.Status.Message = msg
	}
}

func repoReady() func(*velerov1api.ResticRepository) {
	return func(r *velerov1api.ResticRepository) {
		r.Status.Phase = velerov1api.ResticRepositoryPhaseReady
		r.Status.Message = ""
	}
}

// patchResticRepository mutates req with the provided mutate function, and patches it
// through the Kube API. After executing this function, req will be updated with both
// the mutation and the results of the Patch() API call.
func (c *resticRepositoryController) patchResticRepository(req *velerov1api.ResticRepository, mutate func(*velerov1api.ResticRepository)) error {
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
	var patched *velerov1api.ResticRepository
	if patched, err = c.resticRepositoryClient.ResticRepositories(req.Namespace).Patch(context.TODO(), req.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return errors.Wrap(err, "error patching ResticRepository")
	}
	req = patched

	return nil
}
