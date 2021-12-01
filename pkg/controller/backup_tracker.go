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

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
)

// BackupTracker keeps track of in-progress backups.
type BackupTracker interface {
	// Add informs the tracker that a backup is in progress.
	Add(ns, name string, backup *pkgbackup.Request)
	// Delete informs the tracker that a backup is no longer in progress.
	Delete(ns, name string)
	// Contains returns true if the tracker is tracking the backup.
	Contains(ns, name string) bool
	// Get returns the backup for the given namespace/name
	Get(ns, name string) *pkgbackup.Request
	// GetByPhase returns all backups with the given phase
	GetByPhase(phase v1.BackupPhase) []*pkgbackup.Request
}

type backupTracker struct {
	lock    sync.RWMutex
	backups map[string]*pkgbackup.Request
}

// NewBackupTracker returns a new BackupTracker.
func NewBackupTracker() BackupTracker {
	return &backupTracker{
		backups: map[string]*pkgbackup.Request{},
	}
}

func (bt *backupTracker) Add(ns, name string, backup *pkgbackup.Request) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	if backup == nil {
		panic("Cannot add a nil backup to BackupTracker")
	}
	bt.backups[backupTrackerKey(ns, name)] = backup
}

func (bt *backupTracker) Delete(ns, name string) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	delete(bt.backups, backupTrackerKey(ns, name))
}

func (bt *backupTracker) Contains(ns, name string) bool {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	return bt.backups[backupTrackerKey(ns, name)] != nil
}

func (bt *backupTracker) Get(ns, name string) *pkgbackup.Request {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	return bt.backups[backupTrackerKey(ns, name)]
}

func (bt *backupTracker) GetByPhase(phase v1.BackupPhase) []*pkgbackup.Request {
	bt.lock.RLock()
	defer bt.lock.RUnlock()

	foundBackups := []*pkgbackup.Request{}
	for _, backup := range bt.backups {
		if backup.Status.Phase == phase {
			foundBackups = append(foundBackups, backup)
		}
	}
	return foundBackups
}

func backupTrackerKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
