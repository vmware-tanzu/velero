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
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// BackupTracker keeps track of in-progress backups.
type BackupTracker interface {
	// Add informs the tracker that a backup is in progress.
	Add(ns, name string)
	// Delete informs the tracker that a backup is no longer in progress.
	Delete(ns, name string)
	// Contains returns true if the tracker is tracking the backup.
	Contains(ns, name string) bool
}

type backupTracker struct {
	lock    sync.RWMutex
	backups sets.String
}

// NewBackupTracker returns a new BackupTracker.
func NewBackupTracker() BackupTracker {
	return &backupTracker{
		backups: sets.NewString(),
	}
}

func (bt *backupTracker) Add(ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.backups.Insert(backupTrackerKey(ns, name))
}

func (bt *backupTracker) Delete(ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	bt.backups.Delete(backupTrackerKey(ns, name))
}

func (bt *backupTracker) Contains(ns, name string) bool {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	return bt.backups.Has(backupTrackerKey(ns, name))
}

func backupTrackerKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
