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

// resourceDeletionStatusTracker keeps track of in-progress backups.
type ResourceDeletionStatusTracker interface {
	// Add informs the tracker that a polling is in progress to check namespace deletion status.
	Add(kind, ns, name string)
	// Delete informs the tracker that a namespace deletion is completed.
	Delete(kind, ns, name string)
	// Contains returns true if the tracker is tracking the namespace deletion progress.
	Contains(kind, ns, name string) bool
}

type resourceDeletionStatusTracker struct {
	lock                           sync.RWMutex
	isNameSpacePresentInPollingSet sets.Set[string]
}

// NewResourceDeletionStatusTracker returns a new ResourceDeletionStatusTracker.
func NewResourceDeletionStatusTracker() ResourceDeletionStatusTracker {
	return &resourceDeletionStatusTracker{
		isNameSpacePresentInPollingSet: sets.New[string](),
	}
}

func (bt *resourceDeletionStatusTracker) Add(kind, ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.isNameSpacePresentInPollingSet.Insert(resourceDeletionStatusTrackerKey(kind, ns, name))
}

func (bt *resourceDeletionStatusTracker) Delete(kind, ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.isNameSpacePresentInPollingSet.Delete(resourceDeletionStatusTrackerKey(kind, ns, name))
}

func (bt *resourceDeletionStatusTracker) Contains(kind, ns, name string) bool {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	return bt.isNameSpacePresentInPollingSet.Has(resourceDeletionStatusTrackerKey(kind, ns, name))
}

func resourceDeletionStatusTrackerKey(kind, ns, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, ns, name)
}
