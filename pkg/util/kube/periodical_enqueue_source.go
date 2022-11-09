/*
Copyright the Velero contributors.

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

package kube

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func NewPeriodicalEnqueueSource(
	logger logrus.FieldLogger,
	client client.Client,
	objList client.ObjectList,
	period time.Duration,
	option PeriodicalEnqueueSourceOption) *PeriodicalEnqueueSource {
	return &PeriodicalEnqueueSource{
		logger:  logger.WithField("resource", reflect.TypeOf(objList).String()),
		Client:  client,
		objList: objList,
		period:  period,
		option:  option,
	}
}

// PeriodicalEnqueueSource is an implementation of interface sigs.k8s.io/controller-runtime/pkg/source/Source
// It reads the specific resources from Kubernetes/cache and enqueues them into the queue to trigger
// the reconcile logic periodically
type PeriodicalEnqueueSource struct {
	client.Client
	logger  logrus.FieldLogger
	objList client.ObjectList
	period  time.Duration
	option  PeriodicalEnqueueSourceOption
}

type PeriodicalEnqueueSourceOption struct {
	OrderFunc func(objList client.ObjectList) client.ObjectList
}

// Start enqueue items periodically. The predicates only apply to the GenericEvent
func (p *PeriodicalEnqueueSource) Start(ctx context.Context, h handler.EventHandler, q workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
	go wait.Until(func() {
		p.logger.Debug("enqueueing resources ...")
		if err := p.List(ctx, p.objList); err != nil {
			p.logger.WithError(err).Error("error listing resources")
			return
		}
		if meta.LenList(p.objList) == 0 {
			p.logger.Debug("no resources, skip")
			return
		}
		if p.option.OrderFunc != nil {
			p.objList = p.option.OrderFunc(p.objList)
		}
		if err := meta.EachListItem(p.objList, func(object runtime.Object) error {
			obj, ok := object.(client.Object)
			if !ok {
				p.logger.Error("%s's type isn't metav1.Object", object.GetObjectKind().GroupVersionKind().String())
				return nil
			}
			event := event.GenericEvent{Object: obj}
			for _, predicate := range predicates {
				if !predicate.Generic(event) {
					p.logger.Debugf("skip enqueue object %s/%s due to the predicate.", obj.GetNamespace(), obj.GetName())
					return nil
				}
			}

			q.Add(ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				},
			})
			p.logger.Debugf("resource %s/%s enqueued", obj.GetNamespace(), obj.GetName())
			return nil
		}); err != nil {
			p.logger.WithError(err).Error("error enqueueing resources")
			return
		}
	}, p.period, ctx.Done())

	return nil
}

func (p *PeriodicalEnqueueSource) String() string {
	if p.objList != nil {
		return fmt.Sprintf("kind source: %T", p.objList)
	}
	return "kind source: unknown type"
}
