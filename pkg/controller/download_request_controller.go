/*
Copyright 2017 the Heptio Ark contributors.

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

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/util/kube"
)

type downloadRequestController struct {
	*genericController

	downloadRequestClient arkv1client.DownloadRequestsGetter
	downloadRequestLister listers.DownloadRequestLister
	restoreLister         listers.RestoreLister
	clock                 clock.Clock
	createSignedURL       cloudprovider.CreateSignedURLFunc
	backupLocationLister  listers.BackupStorageLocationLister
	backupLister          listers.BackupLister
	newPluginManager      func(logrus.FieldLogger) plugin.Manager
}

// NewDownloadRequestController creates a new DownloadRequestController.
func NewDownloadRequestController(
	downloadRequestClient arkv1client.DownloadRequestsGetter,
	downloadRequestInformer informers.DownloadRequestInformer,
	restoreInformer informers.RestoreInformer,
	backupLocationInformer informers.BackupStorageLocationInformer,
	backupInformer informers.BackupInformer,
	newPluginManager func(logrus.FieldLogger) plugin.Manager,
	logger logrus.FieldLogger,
) Interface {
	c := &downloadRequestController{
		genericController:     newGenericController("downloadrequest", logger),
		downloadRequestClient: downloadRequestClient,
		downloadRequestLister: downloadRequestInformer.Lister(),
		restoreLister:         restoreInformer.Lister(),
		backupLocationLister:  backupLocationInformer.Lister(),
		backupLister:          backupInformer.Lister(),

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		createSignedURL:  cloudprovider.CreateSignedURL,
		newPluginManager: newPluginManager,

		clock: &clock.RealClock{},
	}

	c.syncHandler = c.processDownloadRequest
	c.cacheSyncWaiters = append(
		c.cacheSyncWaiters,
		downloadRequestInformer.Informer().HasSynced,
		restoreInformer.Informer().HasSynced,
		backupLocationInformer.Informer().HasSynced,
		backupInformer.Informer().HasSynced,
	)

	downloadRequestInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err != nil {
					downloadRequest := obj.(*v1.DownloadRequest)
					c.logger.WithError(errors.WithStack(err)).
						WithField("downloadRequest", downloadRequest.Name).
						Error("Error creating queue key, item not added to queue")
					return
				}
				c.queue.Add(key)
			},
		},
	)

	return c
}

// processDownloadRequest is the default per-item sync handler. It generates a pre-signed URL for
// a new DownloadRequest or deletes the DownloadRequest if it has expired.
func (c *downloadRequestController) processDownloadRequest(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processDownloadRequest")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(err).Error("error splitting queue key")
		return nil
	}

	downloadRequest, err := c.downloadRequestLister.DownloadRequests(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find DownloadRequest")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting DownloadRequest")
	}

	switch downloadRequest.Status.Phase {
	case "", v1.DownloadRequestPhaseNew:
		return c.generatePreSignedURL(downloadRequest, log)
	case v1.DownloadRequestPhaseProcessed:
		return c.deleteIfExpired(downloadRequest)
	}

	return nil
}

const signedURLTTL = 10 * time.Minute

// generatePreSignedURL generates a pre-signed URL for downloadRequest, changes the phase to
// Processed, and persists the changes to storage.
func (c *downloadRequestController) generatePreSignedURL(downloadRequest *v1.DownloadRequest, log logrus.FieldLogger) error {
	update := downloadRequest.DeepCopy()

	var (
		directory string
		err       error
	)

	switch downloadRequest.Spec.Target.Kind {
	case v1.DownloadTargetKindRestoreLog, v1.DownloadTargetKindRestoreResults:
		restore, err := c.restoreLister.Restores(downloadRequest.Namespace).Get(downloadRequest.Spec.Target.Name)
		if err != nil {
			return errors.Wrap(err, "error getting Restore")
		}

		directory = restore.Spec.BackupName
	default:
		directory = downloadRequest.Spec.Target.Name
	}

	backup, err := c.backupLister.Backups(downloadRequest.Namespace).Get(directory)
	if err != nil {
		return errors.WithStack(err)
	}

	backupLocation, err := c.backupLocationLister.BackupStorageLocations(backup.Namespace).Get(backup.Spec.StorageLocation)
	if err != nil {
		return errors.WithStack(err)
	}

	pluginManager := c.newPluginManager(log)
	defer pluginManager.CleanupClients()

	objectStore, err := getObjectStoreForLocation(backupLocation, pluginManager)
	if err != nil {
		return errors.WithStack(err)
	}

	update.Status.DownloadURL, err = c.createSignedURL(objectStore, downloadRequest.Spec.Target, backupLocation.Spec.ObjectStorage.Bucket, directory, signedURLTTL)
	if err != nil {
		return err
	}

	update.Status.Phase = v1.DownloadRequestPhaseProcessed
	update.Status.Expiration = metav1.NewTime(c.clock.Now().Add(signedURLTTL))

	_, err = patchDownloadRequest(downloadRequest, update, c.downloadRequestClient)
	return errors.WithStack(err)
}

// deleteIfExpired deletes downloadRequest if it has expired.
func (c *downloadRequestController) deleteIfExpired(downloadRequest *v1.DownloadRequest) error {
	logContext := c.logger.WithField("key", kube.NamespaceAndName(downloadRequest))
	logContext.Info("checking for expiration of DownloadRequest")
	if downloadRequest.Status.Expiration.Time.After(c.clock.Now()) {
		logContext.Debug("DownloadRequest has not expired")
		return nil
	}

	logContext.Debug("DownloadRequest has expired - deleting")
	return errors.WithStack(c.downloadRequestClient.DownloadRequests(downloadRequest.Namespace).Delete(downloadRequest.Name, nil))
}

// resync requeues all the DownloadRequests in the lister's cache. This is mostly to handle deleting
// any expired requests that were not deleted as part of the normal client flow for whatever reason.
func (c *downloadRequestController) resync() {
	list, err := c.downloadRequestLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("error listing download requests")
		return
	}

	for _, dr := range list {
		key, err := cache.MetaNamespaceKeyFunc(dr)
		if err != nil {
			c.logger.WithError(errors.WithStack(err)).WithField("downloadRequest", dr.Name).Error("error generating key for download request")
			continue
		}

		c.queue.Add(key)
	}
}

func patchDownloadRequest(original, updated *v1.DownloadRequest, client arkv1client.DownloadRequestsGetter) (*v1.DownloadRequest, error) {
	origBytes, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original download request")
	}

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated download request")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for download request")
	}

	res, err := client.DownloadRequests(original.Namespace).Patch(original.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching download request")
	}

	return res, nil
}
