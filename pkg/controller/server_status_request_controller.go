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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	arkv1informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	arkv1listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/serverstatusrequest"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
)

const statusRequestResyncPeriod = 5 * time.Minute

type statusRequestController struct {
	*genericController

	client arkv1client.ServerStatusRequestsGetter
	lister arkv1listers.ServerStatusRequestLister
	clock  clock.Clock
}

func NewServerStatusRequestController(
	logger logrus.FieldLogger,
	client arkv1client.ServerStatusRequestsGetter,
	informer arkv1informers.ServerStatusRequestInformer,
) *statusRequestController {
	c := &statusRequestController{
		genericController: newGenericController("serverstatusrequest", logger),
		client:            client,
		lister:            informer.Lister(),

		clock: clock.RealClock{},
	}

	c.syncHandler = c.processItem
	c.cacheSyncWaiters = append(c.cacheSyncWaiters, informer.Informer().HasSynced)

	c.resyncFunc = c.enqueueAllItems
	c.resyncPeriod = statusRequestResyncPeriod

	informer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				req := obj.(*arkv1api.ServerStatusRequest)
				key := kubeutil.NamespaceAndName(req)

				c.logger.WithFields(logrus.Fields{
					"serverStatusRequest": key,
					"phase":               req.Status.Phase,
				}).Debug("Enqueueing server status request")

				c.queue.Add(key)
			},
		},
	)

	return c
}

func (c *statusRequestController) processItem(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processItem")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	log.Debug("Getting ServerStatusRequest")
	req, err := c.lister.ServerStatusRequests(ns).Get(name)
	// server status request no longer exists
	if apierrors.IsNotFound(err) {
		log.WithError(err).Debug("ServerStatusRequest not found")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting ServerStatusRequest")
	}

	return serverstatusrequest.Process(req.DeepCopy(), c.client, c.clock, log)
}

func (c *statusRequestController) enqueueAllItems() {
	items, err := c.lister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(err).Error("Error listing all server status requests")
		return
	}

	for _, req := range items {
		c.enqueue(req)
	}
}
