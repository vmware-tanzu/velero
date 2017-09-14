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

package restore

import (
	"time"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

// how long should we wait for certain objects (e.g. PVs, PVCs) to reach
// their specified conditions before continuing on.
const objectCreateWaitTimeout = 30 * time.Second

// resourceWaiter knows how to wait for a set of registered items to become "ready" (according
// to a provided readyFunc) based on listening to a channel of Events. The correct usage
// of this struct is to construct it, register all of the desired items to wait for via
// RegisterItem, and then to Wait() for them to become ready or the timeout to be exceeded.
type resourceWaiter struct {
	watchChan <-chan watch.Event
	items     sets.String
	readyFunc func(runtime.Unstructured) bool
}

func newResourceWaiter(watchChan <-chan watch.Event, readyFunc func(runtime.Unstructured) bool) *resourceWaiter {
	return &resourceWaiter{
		watchChan: watchChan,
		items:     sets.NewString(),
		readyFunc: readyFunc,
	}
}

// RegisterItem adds the specified key to a list of items to listen for events for.
func (rw *resourceWaiter) RegisterItem(key string) {
	rw.items.Insert(key)
}

// Wait listens for events on the watchChan related to items that have been registered,
// and returns when either all of them have become ready according to readyFunc, or when
// the timeout has been exceeded.
func (rw *resourceWaiter) Wait() error {
	for {
		if rw.items.Len() <= 0 {
			return nil
		}

		timeout := time.NewTimer(objectCreateWaitTimeout)

		select {
		case event := <-rw.watchChan:
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				return errors.Errorf("Unexpected type %T", event.Object)
			}

			if event.Type == watch.Added || event.Type == watch.Modified {
				if rw.items.Has(obj.GetName()) && rw.readyFunc(obj) {
					rw.items.Delete(obj.GetName())
				}
			}
		case <-timeout.C:
			return errors.New("failed to observe all items becoming ready within the timeout")
		}
	}
}
