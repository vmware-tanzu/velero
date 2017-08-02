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

package cloudprovider

import (
	"context"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

// backupCacheBucket holds the backups and error from a GetAllBackups call.
type backupCacheBucket struct {
	backups []*v1.Backup
	error   error
}

// backupCache caches GetAllBackups calls, refreshing them periodically.
type backupCache struct {
	delegate BackupGetter
	lock     sync.RWMutex
	// This doesn't really need to be a map right now, but if we ever move to supporting multiple
	// buckets, this will be ready for it.
	buckets map[string]*backupCacheBucket
}

var _ BackupGetter = &backupCache{}

// NewBackupCache returns a new backup cache that refreshes from delegate every resyncPeriod.
func NewBackupCache(ctx context.Context, delegate BackupGetter, resyncPeriod time.Duration) BackupGetter {
	c := &backupCache{
		delegate: delegate,
		buckets:  make(map[string]*backupCacheBucket),
	}

	// Start the goroutine to refresh all buckets every resyncPeriod. This stops when ctx.Done() is
	// available.
	go wait.Until(c.refresh, resyncPeriod, ctx.Done())

	return c
}

// refresh refreshes all the buckets currently in the cache by doing a live lookup via c.delegate.
func (c *backupCache) refresh() {
	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("refreshing all cached backup lists from object storage")

	for bucketName, bucket := range c.buckets {
		glog.V(4).Infof("refreshing bucket %q", bucketName)
		bucket.backups, bucket.error = c.delegate.GetAllBackups(bucketName)
	}
}

func (c *backupCache) GetAllBackups(bucketName string) ([]*v1.Backup, error) {
	c.lock.RLock()
	bucket, found := c.buckets[bucketName]
	c.lock.RUnlock()
	if found {
		glog.V(4).Infof("returning cached backup list for bucket %q", bucketName)
		return bucket.backups, bucket.error
	}

	glog.V(4).Infof("bucket %q is not in cache - doing a live lookup", bucketName)

	backups, err := c.delegate.GetAllBackups(bucketName)
	c.lock.Lock()
	c.buckets[bucketName] = &backupCacheBucket{backups: backups, error: err}
	c.lock.Unlock()

	return backups, err
}
