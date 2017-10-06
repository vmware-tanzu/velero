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
	"errors"
	"testing"
	"time"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/test"
)

func TestNewBackupCache(t *testing.T) {

	var (
		delegate    = &test.FakeBackupService{}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		logger, _   = testlogger.NewNullLogger()
	)
	defer cancel()

	c := NewBackupCache(ctx, delegate, 100*time.Millisecond, logger)

	// nothing in cache, live lookup
	bucket1 := []*v1.Backup{
		test.NewTestBackup().WithName("backup1").Backup,
		test.NewTestBackup().WithName("backup2").Backup,
	}
	delegate.On("GetAllBackups", "bucket1").Return(bucket1, nil).Once()

	// should be updated via refresh
	updatedBucket1 := []*v1.Backup{
		test.NewTestBackup().WithName("backup2").Backup,
	}
	delegate.On("GetAllBackups", "bucket1").Return(updatedBucket1, nil)

	// nothing in cache, live lookup
	bucket2 := []*v1.Backup{
		test.NewTestBackup().WithName("backup5").Backup,
		test.NewTestBackup().WithName("backup6").Backup,
	}
	delegate.On("GetAllBackups", "bucket2").Return(bucket2, nil).Once()

	// should be updated via refresh
	updatedBucket2 := []*v1.Backup{
		test.NewTestBackup().WithName("backup7").Backup,
	}
	delegate.On("GetAllBackups", "bucket2").Return(updatedBucket2, nil)

	backups, err := c.GetAllBackups("bucket1")
	assert.Equal(t, bucket1, backups)
	assert.NoError(t, err)

	backups, err = c.GetAllBackups("bucket2")
	assert.Equal(t, bucket2, backups)
	assert.NoError(t, err)

	var done1, done2 bool
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timed out")
		default:
			if done1 && done2 {
				return
			}
		}

		backups, err = c.GetAllBackups("bucket1")
		if len(backups) == 1 {
			if assert.Equal(t, updatedBucket1[0], backups[0]) {
				done1 = true
			}
		}

		backups, err = c.GetAllBackups("bucket2")
		if len(backups) == 1 {
			if assert.Equal(t, updatedBucket2[0], backups[0]) {
				done2 = true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestBackupCacheRefresh(t *testing.T) {
	var (
		delegate  = &test.FakeBackupService{}
		logger, _ = testlogger.NewNullLogger()
	)

	c := &backupCache{
		delegate: delegate,
		buckets: map[string]*backupCacheBucket{
			"bucket1": {},
			"bucket2": {},
		},
		logger: logger,
	}

	bucket1 := []*v1.Backup{
		test.NewTestBackup().WithName("backup1").Backup,
		test.NewTestBackup().WithName("backup2").Backup,
	}
	delegate.On("GetAllBackups", "bucket1").Return(bucket1, nil)

	delegate.On("GetAllBackups", "bucket2").Return(nil, errors.New("bad"))

	c.refresh()

	assert.Equal(t, bucket1, c.buckets["bucket1"].backups)
	assert.NoError(t, c.buckets["bucket1"].error)

	assert.Empty(t, c.buckets["bucket2"].backups)
	assert.EqualError(t, c.buckets["bucket2"].error, "bad")
}

func TestBackupCacheGetAllBackupsUsesCacheIfPresent(t *testing.T) {
	var (
		delegate  = &test.FakeBackupService{}
		logger, _ = testlogger.NewNullLogger()
		bucket1   = []*v1.Backup{
			test.NewTestBackup().WithName("backup1").Backup,
			test.NewTestBackup().WithName("backup2").Backup,
		}
	)

	c := &backupCache{
		delegate: delegate,
		buckets: map[string]*backupCacheBucket{
			"bucket1": {
				backups: bucket1,
			},
		},
		logger: logger,
	}

	bucket2 := []*v1.Backup{
		test.NewTestBackup().WithName("backup3").Backup,
		test.NewTestBackup().WithName("backup4").Backup,
	}

	delegate.On("GetAllBackups", "bucket2").Return(bucket2, nil)

	backups, err := c.GetAllBackups("bucket1")
	assert.Equal(t, bucket1, backups)
	assert.NoError(t, err)

	backups, err = c.GetAllBackups("bucket2")
	assert.Equal(t, bucket2, backups)
	assert.NoError(t, err)
}
