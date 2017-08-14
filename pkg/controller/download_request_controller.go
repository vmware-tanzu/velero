/*
Copyright 2017 Heptio Inc.

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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
)

type downloadRequestController struct {
	downloadRequestClient       arkv1client.DownloadRequestsGetter
	downloadRequestLister       listers.DownloadRequestLister
	downloadRequestListerSynced cache.InformerSynced
	backupLister                listers.BackupLister
	backupListerSynced          cache.InformerSynced

	backupService cloudprovider.BackupService
	bucket        string

	syncHandler func(key string) error
	queue       workqueue.RateLimitingInterface

	clock clock.Clock
}

// NewDownloadRequestController creates a new DownloadRequestController.
func NewDownloadRequestController(
	downloadRequestClient arkv1client.DownloadRequestsGetter,
	downloadRequestInformer informers.DownloadRequestInformer,
	backupInformer informers.BackupInformer,
	backupService cloudprovider.BackupService,
	bucket string,
) Interface {
	c := &downloadRequestController{
		downloadRequestClient:       downloadRequestClient,
		downloadRequestLister:       downloadRequestInformer.Lister(),
		downloadRequestListerSynced: downloadRequestInformer.Informer().HasSynced,
		backupLister:                backupInformer.Lister(),
		backupListerSynced:          backupInformer.Informer().HasSynced,

		backupService: backupService,
		bucket:        bucket,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "backup"),

		clock: &clock.RealClock{},
	}

	c.syncHandler = c.processDownloadRequest

	downloadRequestInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err != nil {
					runtime.HandleError(fmt.Errorf("error creating queue key for %#v: %v", obj, err))
					return
				}
				c.queue.Add(key)
			},
		},
	)

	return c
}

// Run is a blocking function that runs the specified number of worker goroutines
// to process items in the work queue. It will return when it receives on the
// ctx.Done() channel.
func (c *downloadRequestController) Run(ctx context.Context, numWorkers int) error {
	var wg sync.WaitGroup

	defer func() {
		glog.Infof("Waiting for workers to finish their work")

		c.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		glog.Infof("All workers have finished")
	}()

	glog.Info("Starting DownloadRequestController")
	defer glog.Infof("Shutting down DownloadRequestController")

	glog.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.downloadRequestListerSynced, c.backupListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	glog.Info("Caches are synced")

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			wait.Until(c.runWorker, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	wg.Add(1)
	go func() {
		wait.Until(c.resync, time.Minute, ctx.Done())
		wg.Done()
	}()

	<-ctx.Done()

	return nil
}

// runWorker runs a worker until the controller's queue indicates it's time to shut down.
func (c *downloadRequestController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem processes a single item from the queue.
func (c *downloadRequestController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// always call done on this item, since if it fails we'll add
	// it back with rate-limiting below
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		// If you had no error, tell the queue to stop tracking history for your key. This will reset
		// things like failure counts for per-item rate limiting.
		c.queue.Forget(key)
		return true
	}

	glog.Errorf("syncHandler error: %v", err)
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	c.queue.AddRateLimited(key)

	return true
}

// processDownloadRequest is the default per-item sync handler. It generates a pre-signed URL for
// a new DownloadRequest or deletes the DownloadRequest if it has expired.
func (c *downloadRequestController) processDownloadRequest(key string) error {
	glog.V(4).Infof("processDownloadRequest for key %q", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.V(4).Infof("error splitting key %q: %v", key, err)
		return err
	}

	downloadRequest, err := c.downloadRequestLister.DownloadRequests(ns).Get(name)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("unable to find DownloadRequest %q", key)
		return nil
	}
	if err != nil {
		glog.V(4).Infof("error getting DownloadRequest %q: %v", key, err)
		return err
	}

	switch downloadRequest.Status.Phase {
	case "", v1.DownloadRequestPhaseNew:
		return c.generatePreSignedURL(downloadRequest)
	case v1.DownloadRequestPhaseProcessed:
		return c.deleteIfExpired(downloadRequest)
	}

	return nil
}

const signedURLTTL = 10 * time.Minute

// generatePreSignedURL generates a pre-signed URL for downloadRequest, changes the phase to
// Processed, and persists the changes to storage.
func (c *downloadRequestController) generatePreSignedURL(downloadRequest *v1.DownloadRequest) error {
	switch downloadRequest.Spec.Target.Kind {
	case v1.DownloadTargetKindBackupLog:
		update, err := cloneDownloadRequest(downloadRequest)
		if err != nil {
			return err
		}

		update.Status.DownloadURL, err = c.backupService.CreateBackupLogSignedURL(c.bucket, update.Spec.Target.Name, signedURLTTL)
		if err != nil {
			return err
		}

		update.Status.Phase = v1.DownloadRequestPhaseProcessed
		update.Status.Expiration = metav1.NewTime(c.clock.Now().Add(signedURLTTL))

		_, err = c.downloadRequestClient.DownloadRequests(update.Namespace).Update(update)
		return err
	}

	return fmt.Errorf("unsupported download target kind %q", downloadRequest.Spec.Target.Kind)
}

// deleteIfExpired deletes downloadRequest if it has expired.
func (c *downloadRequestController) deleteIfExpired(downloadRequest *v1.DownloadRequest) error {
	glog.V(4).Infof("checking for expiration of %s/%s", downloadRequest.Namespace, downloadRequest.Name)
	if downloadRequest.Status.Expiration.Time.Before(c.clock.Now()) {
		glog.V(4).Infof("%s/%s has not expired", downloadRequest.Namespace, downloadRequest.Name)
		return nil
	}

	glog.V(4).Infof("%s/%s has expired - deleting", downloadRequest.Namespace, downloadRequest.Name)
	return c.downloadRequestClient.DownloadRequests(downloadRequest.Namespace).Delete(downloadRequest.Name, nil)
}

// resync requeues all the DownloadRequests in the lister's cache. This is mostly to handle deleting
// any expired requests that were not deleted as part of the normal client flow for whatever reason.
func (c *downloadRequestController) resync() {
	list, err := c.downloadRequestLister.List(labels.Everything())
	if err != nil {
		runtime.HandleError(fmt.Errorf("error listing download requests: %v", err))
		return
	}

	for _, dr := range list {
		key, err := cache.MetaNamespaceKeyFunc(dr)
		if err != nil {
			runtime.HandleError(fmt.Errorf("error generating key for download request %#v: %v", dr, err))
			continue
		}

		c.queue.Add(key)
	}
}

// cloneDownloadRequest makes a deep copy of in.
func cloneDownloadRequest(in *v1.DownloadRequest) (*v1.DownloadRequest, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, err
	}

	out, ok := clone.(*v1.DownloadRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}
