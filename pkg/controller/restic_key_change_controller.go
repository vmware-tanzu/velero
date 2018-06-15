/*
Copyright 2018 the Heptio Ark contributors.

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/restic"
)

type resticKeyChangeController struct {
	*genericController

	namespace              string
	secretLister           corev1listers.SecretLister
	secretClient           corev1client.SecretsGetter
	resticRepositoryClient arkv1client.ResticRepositoriesGetter
	resticRepositoryLister listers.ResticRepositoryLister
	repositoryManager      restic.RepositoryManager
}

// NewResticKeyChangeController creates a new restic key-change controller.
func NewResticKeyChangeController(
	logger logrus.FieldLogger,
	namespace string,
	secretInformer cache.SharedIndexInformer,
	secretClient corev1client.SecretsGetter,
	resticRepositoryInformer informers.ResticRepositoryInformer,
	resticRepositoryClient arkv1client.ResticRepositoriesGetter,
	repositoryManager restic.RepositoryManager,
) Interface {
	c := &resticKeyChangeController{
		genericController:      newGenericController("restic-key-change", logger),
		namespace:              namespace,
		secretClient:           secretClient,
		secretLister:           corev1listers.NewSecretLister(secretInformer.GetIndexer()),
		resticRepositoryClient: resticRepositoryClient,
		resticRepositoryLister: resticRepositoryInformer.Lister(),
		repositoryManager:      repositoryManager,
	}

	c.syncHandler = c.processQueueItem
	c.cacheSyncWaiters = append(c.cacheSyncWaiters, secretInformer.HasSynced, resticRepositoryInformer.Informer().HasSynced)

	secretInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueIfKeyChange,
			UpdateFunc: func(_, obj interface{}) {
				c.enqueueIfKeyChange(obj)
			},
		},
	)

	return c
}

func shouldProcess(secret *corev1.Secret) bool {
	// we need to process a secret if it's got data in either the "new-key"
	// or "old-key" secret slots
	if _, ok := secret.Data[restic.NewCredentialsKey]; ok {
		return true
	}

	if _, ok := secret.Data[restic.OldCredentialsKey]; ok {
		return true
	}

	return false
}

func (c *resticKeyChangeController) enqueueIfKeyChange(obj interface{}) {
	secret := obj.(*corev1.Secret)

	if shouldProcess(secret) {
		c.enqueue(obj)
	}
}

func (c *resticKeyChangeController) processQueueItem(key string) error {
	c.logger.WithField("key", key).Debug("Running processQueueItem")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.WithField("key", key).WithError(errors.WithStack(err)).Error("error splitting queue key")
		return nil
	}
	log := c.logger.WithField("namespace", ns).WithField("name", name)

	secret, err := c.secretLister.Secrets(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find secret")
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "error getting secret")
	}

	// Don't mutate the shared cache
	secretCopy := secret.DeepCopy()

	return c.processKeyChange(secretCopy, log)
}

func (c *resticKeyChangeController) processKeyChange(secret *corev1.Secret, log logrus.FieldLogger) error {
	// check again if we should process because secret may have changed
	// since it was added to the queue
	if !shouldProcess(secret) {
		return nil
	}

	log.Info("Processing repository key change")

	if err := c.repositoryManager.ChangeKey(secret.Namespace); err != nil {
		log.WithError(err).Error("error processing repository key change")
		return err
	}

	repo, err := c.resticRepositoryLister.ResticRepositories(c.namespace).Get(secret.Namespace)
	if err != nil {
		log.WithError(err).Error("error getting repo")
		return nil
	}

	if err := patchRepo(repo, func(r *arkv1api.ResticRepository) {
		r.Status.LastKeyChangeTime = metav1.Time{Time: time.Now()}
	}, c.resticRepositoryClient); err != nil {
		log.WithError(err).Error("error patching repo")
		return nil
	}

	return nil
}

func patchRepo(repo *arkv1api.ResticRepository, mutate func(r *arkv1api.ResticRepository), repoClient arkv1client.ResticRepositoriesGetter) error {
	original, err := json.Marshal(repo)
	if err != nil {
		return errors.Wrapf(err, "error marshalling repo to JSON")
	}

	mutate(repo)

	updated, err := json.Marshal(repo)
	if err != nil {
		return errors.Wrapf(err, "error marshalling updated repo to JSON")
	}

	patch, err := jsonpatch.CreateMergePatch(original, updated)
	if err != nil {
		return errors.Wrapf(err, "error creating merge patch")
	}

	if repo, err = repoClient.ResticRepositories(repo.Namespace).Patch(repo.Name, types.MergePatchType, patch); err != nil {
		return errors.Wrap(err, "error patching repo")
	}

	return nil
}
