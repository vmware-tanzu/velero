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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/util/encode"
)

const backupVersion = 1

type backupController struct {
	backupper        backup.Backupper
	backupService    cloudprovider.BackupService
	bucket           string
	pvProviderExists bool

	lister       listers.BackupLister
	listerSynced cache.InformerSynced
	client       arkv1client.BackupsGetter
	syncHandler  func(backupName string) error
	queue        workqueue.RateLimitingInterface

	clock clock.Clock
}

func NewBackupController(
	backupInformer informers.BackupInformer,
	client arkv1client.BackupsGetter,
	backupper backup.Backupper,
	backupService cloudprovider.BackupService,
	bucket string,
	pvProviderExists bool,
) Interface {
	c := &backupController{
		backupper:        backupper,
		backupService:    backupService,
		bucket:           bucket,
		pvProviderExists: pvProviderExists,

		lister:       backupInformer.Lister(),
		listerSynced: backupInformer.Informer().HasSynced,
		client:       client,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "backup"),

		clock: &clock.RealClock{},
	}

	c.syncHandler = c.processBackup

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				backup := obj.(*api.Backup)

				switch backup.Status.Phase {
				case "", api.BackupPhaseNew:
					// only process new backups
				default:
					glog.V(4).Infof("Backup %s/%s has phase %s - skipping", backup.Namespace, backup.Name, backup.Status.Phase)
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(backup)
				if err != nil {
					glog.Errorf("error creating queue key for %#v: %v", backup, err)
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
func (controller *backupController) Run(ctx context.Context, numWorkers int) error {
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

	glog.Info("Starting BackupController")
	defer glog.Infof("Shutting down BackupController")

	glog.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.listerSynced) {
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

func (controller *backupController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for controller.processNextWorkItem() {
	}
}

func (controller *backupController) processNextWorkItem() bool {
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

func (controller *backupController) processBackup(key string) error {
	glog.V(4).Infof("processBackup for key %q", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.V(4).Infof("error splitting key %q: %v", key, err)
		return err
	}

	glog.V(4).Infof("Getting backup %s", key)
	backup, err := controller.lister.Backups(ns).Get(name)
	if err != nil {
		glog.V(4).Infof("error getting backup %s: %v", key, err)
		return err
	}

	// TODO I think this is now unnecessary. We only initially place
	// item with Phase = ("" | New) into the queue. Items will only get
	// re-queued if syncHandler returns an error, which will only
	// happen if there's an error updating Phase from its initial
	// state to something else. So any time it's re-queued it will
	// still have its initial state, which we've already confirmed
	// is ("" | New)
	switch backup.Status.Phase {
	case "", api.BackupPhaseNew:
		// only process new backups
	default:
		return nil
	}

	glog.V(4).Infof("Cloning backup %s", key)
	// don't modify items in the cache
	backup, err = cloneBackup(backup)
	if err != nil {
		glog.V(4).Infof("error cloning backup %s: %v", key, err)
		return err
	}

	// set backup version
	backup.Status.Version = backupVersion

	// included resources defaulting
	if len(backup.Spec.IncludedResources) == 0 {
		backup.Spec.IncludedResources = []string{"*"}
	}

	// included namespace defaulting
	if len(backup.Spec.IncludedNamespaces) == 0 {
		backup.Spec.IncludedNamespaces = []string{"*"}
	}

	// calculate expiration
	if backup.Spec.TTL.Duration > 0 {
		backup.Status.Expiration = metav1.NewTime(controller.clock.Now().Add(backup.Spec.TTL.Duration))
	}

	// validation
	if backup.Status.ValidationErrors = controller.getValidationErrors(backup); len(backup.Status.ValidationErrors) > 0 {
		backup.Status.Phase = api.BackupPhaseFailedValidation
	} else {
		backup.Status.Phase = api.BackupPhaseInProgress
	}

	// update status
	updatedBackup, err := controller.client.Backups(ns).Update(backup)
	if err != nil {
		glog.V(4).Infof("error updating status to %s: %v", backup.Status.Phase, err)
		return err
	}
	backup = updatedBackup

	if backup.Status.Phase == api.BackupPhaseFailedValidation {
		return nil
	}

	glog.V(4).Infof("running backup for %s", key)
	// execution & upload of backup
	if err := controller.runBackup(backup, controller.bucket); err != nil {
		glog.V(4).Infof("backup %s failed: %v", key, err)
		backup.Status.Phase = api.BackupPhaseFailed
	}

	glog.V(4).Infof("updating backup %s final status", key)
	if _, err = controller.client.Backups(ns).Update(backup); err != nil {
		glog.V(4).Infof("error updating backup %s final status: %v", key, err)
	}

	return nil
}

func cloneBackup(in interface{}) (*api.Backup, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, err
	}

	out, ok := clone.(*api.Backup)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}

func (controller *backupController) getValidationErrors(itm *api.Backup) []string {
	var validationErrors []string

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedResources, itm.Spec.ExcludedResources) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedNamespaces, itm.Spec.ExcludedNamespaces) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	if !controller.pvProviderExists && itm.Spec.SnapshotVolumes != nil && *itm.Spec.SnapshotVolumes {
		validationErrors = append(validationErrors, "Server is not configured for PV snapshots")
	}

	return validationErrors
}

func (controller *backupController) runBackup(backup *api.Backup, bucket string) error {
	backupFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}

	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}

	defer func() {
		var errs []error
		errs = append(errs, err)

		if err := backupFile.Close(); err != nil {
			errs = append(errs, err)
		}

		if err := os.Remove(backupFile.Name()); err != nil {
			errs = append(errs, err)
		}

		if err := logFile.Close(); err != nil {
			errs = append(errs, err)
		}

		if err := os.Remove(logFile.Name()); err != nil {
			errs = append(errs, err)
		}

		err = kuberrs.NewAggregate(errs)
	}()

	if err := controller.backupper.Backup(backup, backupFile, logFile); err != nil {
		return err
	}

	// note: updating this here so the uploaded JSON shows "completed". If
	// the upload fails, we'll alter the phase in the calling func.
	glog.V(4).Infof("backup %s/%s completed", backup.Namespace, backup.Name)
	backup.Status.Phase = api.BackupPhaseCompleted

	buf := new(bytes.Buffer)
	if err := encode.EncodeTo(backup, "json", buf); err != nil {
		return err
	}

	// re-set the files' offset to 0 for reading
	if _, err = backupFile.Seek(0, 0); err != nil {
		return err
	}
	if _, err = logFile.Seek(0, 0); err != nil {
		return err
	}

	return controller.backupService.UploadBackup(bucket, backup.Name, bytes.NewReader(buf.Bytes()), backupFile, logFile)
}
