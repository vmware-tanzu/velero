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

package kube

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// NamespaceDeletionStatusTracker keeps track of in-progress backups.
type NamespaceDeletionStatusTracker interface {
	// Add informs the tracker that a polling is in progress to check namespace deletion status.
	Add(ns, name string)
	// Delete informs the tracker that a namespace deletion is completed.
	Delete(ns, name string)
	// Contains returns true if the tracker is tracking the namespace deletion progress.
	Contains(ns, name string) bool
}

type namespaceDeletionStatusTracker struct {
	lock                           sync.RWMutex
	isNameSpacePresentInPollingSet sets.Set[string]
}

// NewNamespaceDeletionStatusTracker returns a new NamespaceDeletionStatusTracker.
func NewNamespaceDeletionStatusTracker() NamespaceDeletionStatusTracker {
	return &namespaceDeletionStatusTracker{
		isNameSpacePresentInPollingSet: sets.New[string](),
	}
}

func (bt *namespaceDeletionStatusTracker) Add(ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.isNameSpacePresentInPollingSet.Insert(namespaceDeletionStatusTrackerKey(ns, name))
}

func (bt *namespaceDeletionStatusTracker) Delete(ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.isNameSpacePresentInPollingSet.Delete(namespaceDeletionStatusTrackerKey(ns, name))
}

func (bt *namespaceDeletionStatusTracker) Contains(ns, name string) bool {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	return bt.isNameSpacePresentInPollingSet.Has(namespaceDeletionStatusTrackerKey(ns, name))
}

func namespaceDeletionStatusTrackerKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
