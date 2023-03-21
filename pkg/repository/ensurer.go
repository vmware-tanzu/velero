/*
Copyright 2018, 2019 the Velero contributors.

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

package repository

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// RepositoryEnsurer ensures that backup repositories are created and ready.
type RepositoryEnsurer struct {
	log        logrus.FieldLogger
	repoClient client.Client

	// repoLocksMu synchronizes reads/writes to the repoLocks map itself
	// since maps are not threadsafe.
	repoLocksMu     sync.Mutex
	repoLocks       map[BackupRepositoryKey]*sync.Mutex
	resourceTimeout time.Duration
}

func NewRepositoryEnsurer(repoClient client.Client, log logrus.FieldLogger, resourceTimeout time.Duration) *RepositoryEnsurer {
	return &RepositoryEnsurer{
		log:             log,
		repoClient:      repoClient,
		repoLocks:       make(map[BackupRepositoryKey]*sync.Mutex),
		resourceTimeout: resourceTimeout,
	}
}

func (r *RepositoryEnsurer) EnsureRepo(ctx context.Context, namespace, volumeNamespace, backupLocation, repositoryType string) (*velerov1api.BackupRepository, error) {
	if volumeNamespace == "" || backupLocation == "" || repositoryType == "" {
		return nil, errors.Errorf("wrong parameters, namespace %q, backup storage location %q, repository type %q", volumeNamespace, backupLocation, repositoryType)
	}

	backupRepoKey := BackupRepositoryKey{volumeNamespace, backupLocation, repositoryType}

	log := r.log.WithField("volumeNamespace", volumeNamespace).WithField("backupLocation", backupLocation).WithField("repositoryType", repositoryType)

	// It's only safe to have one instance of this method executing concurrently for a
	// given BackupRepositoryKey, so synchronize based on that. It's fine
	// to run concurrently for *different* BackupRepositoryKey. If you had 2 goroutines
	// running this for the same inputs, both might find no BackupRepository exists, then
	// both would create new ones for the same BackupRepositoryKey.
	//
	// This issue could probably be avoided if we had a deterministic name for
	// each BackupRepository and we just tried to create it, checked for an
	// AlreadyExists err, and then waited for it to be ready. However, there are
	// already repositories in the wild with non-deterministic names (i.e. using
	// GenerateName) which poses a backwards compatibility problem.
	log.Debug("Acquiring lock")

	repoMu := r.repoLock(backupRepoKey)
	repoMu.Lock()
	defer func() {
		repoMu.Unlock()
		log.Debug("Released lock")
	}()

	repo, err := GetBackupRepository(ctx, r.repoClient, namespace, backupRepoKey, true)
	if err == nil {
		log.Debug("Ready repository found")
		return repo, nil
	}

	if !isBackupRepositoryNotFoundError(err) {
		return nil, errors.WithStack(err)
	}

	log.Debug("No repository found, creating one")

	// no repo found: create one and wait for it to be ready
	return r.createBackupRepositoryAndWait(ctx, namespace, backupRepoKey)
}

func (r *RepositoryEnsurer) repoLock(key BackupRepositoryKey) *sync.Mutex {
	r.repoLocksMu.Lock()
	defer r.repoLocksMu.Unlock()

	if r.repoLocks[key] == nil {
		r.repoLocks[key] = new(sync.Mutex)
	}

	return r.repoLocks[key]
}

func (r *RepositoryEnsurer) createBackupRepositoryAndWait(ctx context.Context, namespace string, backupRepoKey BackupRepositoryKey) (*velerov1api.BackupRepository, error) {
	toCreate := newBackupRepository(namespace, backupRepoKey)
	if err := r.repoClient.Create(ctx, toCreate, &client.CreateOptions{}); err != nil {
		return nil, errors.Wrap(err, "unable to create backup repository resource")
	}

	var repo *velerov1api.BackupRepository
	checkFunc := func(ctx context.Context) (bool, error) {
		found, err := GetBackupRepository(ctx, r.repoClient, namespace, backupRepoKey, true)
		if err == nil {
			repo = found
			return true, nil
		} else if isBackupRepositoryNotFoundError(err) || isBackupRepositoryNotProvisionedError(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	err := wait.PollWithContext(ctx, time.Millisecond*500, r.resourceTimeout, checkFunc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait BackupRepository")
	} else {
		return repo, nil
	}
}
