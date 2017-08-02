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
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/restore"
)

type restoreController struct {
	restoreClient arkv1client.RestoresGetter
	backupClient  arkv1client.BackupsGetter
	restorer      restore.Restorer
	backupService cloudprovider.BackupService
	bucket        string

	backupLister        listers.BackupLister
	backupListerSynced  cache.InformerSynced
	restoreLister       listers.RestoreLister
	restoreListerSynced cache.InformerSynced
	syncHandler         func(restoreName string) error
	queue               workqueue.RateLimitingInterface
}

func NewRestoreController(
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	backupClient arkv1client.BackupsGetter,
	restorer restore.Restorer,
	backupService cloudprovider.BackupService,
	bucket string,
	backupInformer informers.BackupInformer,
) Interface {
	c := &restoreController{
		restoreClient:       restoreClient,
		backupClient:        backupClient,
		restorer:            restorer,
		backupService:       backupService,
		bucket:              bucket,
		backupLister:        backupInformer.Lister(),
		backupListerSynced:  backupInformer.Informer().HasSynced,
		restoreLister:       restoreInformer.Lister(),
		restoreListerSynced: restoreInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "restore"),
	}

	c.syncHandler = c.processRestore

	restoreInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				restore := obj.(*api.Restore)

				switch restore.Status.Phase {
				case "", api.RestorePhaseNew:
					// only process new restores
				default:
					glog.V(4).Infof("Restore %s/%s has phase %s - skipping", restore.Namespace, restore.Name, restore.Status.Phase)
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(restore)
				if err != nil {
					glog.Errorf("error creating queue key for %#v: %v", restore, err)
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
func (controller *restoreController) Run(ctx context.Context, numWorkers int) error {
	var wg sync.WaitGroup

	defer func() {
		glog.Infof("Waiting for workers to finish their work")

		controller.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		glog.Infof("All workers have finished")
	}()

	glog.Info("Starting RestoreController")
	defer glog.Info("Shutting down RestoreController")

	glog.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.backupListerSynced, controller.restoreListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	glog.Info("Caches are synced")

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			wait.Until(controller.runWorker, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	<-ctx.Done()

	return nil
}

func (controller *restoreController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for controller.processNextWorkItem() {
	}
}

func (controller *restoreController) processNextWorkItem() bool {
	key, quit := controller.queue.Get()
	if quit {
		return false
	}
	// always call done on this item, since if it fails we'll add
	// it back with rate-limiting below
	defer controller.queue.Done(key)

	err := controller.syncHandler(key.(string))
	if err == nil {
		// If you had no error, tell the queue to stop tracking history for your key. This will reset
		// things like failure counts for per-item rate limiting.
		controller.queue.Forget(key)
		return true
	}

	glog.Errorf("syncHandler error: %v", err)
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	controller.queue.AddRateLimited(key)

	return true
}

func (controller *restoreController) processRestore(key string) error {
	glog.V(4).Infof("processRestore for key %q", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.V(4).Infof("error splitting key %q: %v", key, err)
		return err
	}

	glog.V(4).Infof("Getting restore %s", key)
	restore, err := controller.restoreLister.Restores(ns).Get(name)
	if err != nil {
		glog.V(4).Infof("error getting restore %s: %v", key, err)
		return err
	}

	// TODO I think this is now unnecessary. We only initially place
	// item with Phase = ("" | New) into the queue. Items will only get
	// re-queued if syncHandler returns an error, which will only
	// happen if there's an error updating Phase from its initial
	// state to something else. So any time it's re-queued it will
	// still have its initial state, which we've already confirmed
	// is ("" | New)
	switch restore.Status.Phase {
	case "", api.RestorePhaseNew:
		// only process new restores
	default:
		return nil
	}

	glog.V(4).Infof("Cloning restore %s", key)
	// don't modify items in the cache
	restore, err = cloneRestore(restore)
	if err != nil {
		glog.V(4).Infof("error cloning restore %s: %v", key, err)
		return err
	}

	// validation
	if restore.Status.ValidationErrors = controller.getValidationErrors(restore); len(restore.Status.ValidationErrors) > 0 {
		restore.Status.Phase = api.RestorePhaseFailedValidation
	} else {
		restore.Status.Phase = api.RestorePhaseInProgress
	}

	if len(restore.Spec.Namespaces) == 0 {
		restore.Spec.Namespaces = []string{"*"}
	}

	// update status
	updatedRestore, err := controller.restoreClient.Restores(ns).Update(restore)
	if err != nil {
		glog.V(4).Infof("error updating status to %s: %v", restore.Status.Phase, err)
		return err
	}
	restore = updatedRestore

	if restore.Status.Phase == api.RestorePhaseFailedValidation {
		return nil
	}

	glog.V(4).Infof("running restore for %s", key)
	// execution & upload of restore
	restore.Status.Warnings, restore.Status.Errors = controller.runRestore(restore, controller.bucket)

	glog.V(4).Infof("restore %s completed", key)
	restore.Status.Phase = api.RestorePhaseCompleted

	glog.V(4).Infof("updating restore %s final status", key)
	if _, err = controller.restoreClient.Restores(ns).Update(restore); err != nil {
		glog.V(4).Infof("error updating restore %s final status: %v", key, err)
	}

	return nil
}

func cloneRestore(in interface{}) (*api.Restore, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, err
	}

	out, ok := clone.(*api.Restore)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}

func (controller *restoreController) getValidationErrors(itm *api.Restore) []string {
	var validationErrors []string

	if itm.Spec.BackupName == "" {
		validationErrors = append(validationErrors, "BackupName must be non-empty and correspond to the name of a backup in object storage.")
	}

	return validationErrors
}

func (controller *restoreController) runRestore(restore *api.Restore, bucket string) (warnings, errors api.RestoreResult) {
	backup, err := controller.backupLister.Backups(api.DefaultNamespace).Get(restore.Spec.BackupName)
	if err != nil {
		glog.Errorf("error getting backup: %v", err)
		errors.Cluster = append(errors.Ark, err.Error())
		return
	}

	tmpFile, err := downloadToTempFile(restore.Spec.BackupName, controller.backupService, bucket)
	if err != nil {
		glog.Errorf("error downloading backup: %v", err)
		errors.Cluster = append(errors.Ark, err.Error())
		return
	}

	defer func() {
		if err := tmpFile.Close(); err != nil {
			errors.Cluster = append(errors.Ark, err.Error())
		}

		if err := os.Remove(tmpFile.Name()); err != nil {
			errors.Cluster = append(errors.Ark, err.Error())
		}
	}()

	return controller.restorer.Restore(restore, backup, tmpFile)
}

func downloadToTempFile(backupName string, backupService cloudprovider.BackupService, bucket string) (*os.File, error) {
	readCloser, err := backupService.DownloadBackup(bucket, backupName)
	if err != nil {
		return nil, err
	}
	defer readCloser.Close()

	file, err := ioutil.TempFile("", backupName)
	if err != nil {
		return nil, err
	}

	n, err := io.Copy(file, readCloser)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("copied %d bytes", n)

	if _, err := file.Seek(0, 0); err != nil {
		glog.V(4).Infof("error seeking: %v", err)
		return nil, err
	}

	return file, nil
}
