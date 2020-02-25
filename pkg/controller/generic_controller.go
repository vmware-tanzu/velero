/*
Copyright 2018 the Velero contributors.

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
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type genericController struct {
	name             string
	queue            workqueue.RateLimitingInterface
	logger           logrus.FieldLogger
	syncHandler      func(key string) error
	resyncFunc       func()
	resyncPeriod     time.Duration
	cacheSyncWaiters []cache.InformerSynced
}

func newGenericController(name string, logger logrus.FieldLogger) *genericController {
	c := &genericController{
		name:   name,
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		logger: logger.WithField("controller", name),
	}

	return c
}

// Run is a blocking function that runs the specified number of worker goroutines
// to process items in the work queue. It will return when it receives on the
// ctx.Done() channel.
func (c *genericController) Run(ctx context.Context, numWorkers int) error {
	if c.syncHandler == nil && c.resyncFunc == nil {
		// programmer error
		panic("at least one of syncHandler or resyncFunc is required")
	}

	var wg sync.WaitGroup

	defer func() {
		c.logger.Info("Waiting for workers to finish their work")

		c.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		c.logger.Info("All workers have finished")

	}()

	c.logger.Info("Starting controller")
	defer c.logger.Info("Shutting down controller")

	// only want to log about cache sync waiters if there are any
	if len(c.cacheSyncWaiters) > 0 {
		c.logger.Info("Waiting for caches to sync")
		if !cache.WaitForCacheSync(ctx.Done(), c.cacheSyncWaiters...) {
			return errors.New("timed out waiting for caches to sync")
		}
		c.logger.Info("Caches are synced")
	}

	if c.syncHandler != nil {
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func() {
				wait.Until(c.runWorker, time.Second, ctx.Done())
				wg.Done()
			}()
		}
	}

	if c.resyncFunc != nil {
		if c.resyncPeriod == 0 {
			// Programmer error
			panic("non-zero resyncPeriod is required")
		}

		wg.Add(1)
		go func() {
			wait.Until(c.resyncFunc, c.resyncPeriod, ctx.Done())
			wg.Done()
		}()
	}

	<-ctx.Done()

	return nil
}

func (c *genericController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for c.processNextWorkItem() {
	}
}

func (c *genericController) processNextWorkItem() bool {
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

	c.logger.WithError(err).WithField("key", key).Error("Error in syncHandler, re-adding item to queue")
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	c.queue.AddRateLimited(key)

	return true
}

func (c *genericController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).
			Error("Error creating queue key, item not added to queue")
		return
	}

	c.queue.Add(key)
}

func (c *genericController) enqueueSecond(_, obj interface{}) {
	c.enqueue(obj)
}
