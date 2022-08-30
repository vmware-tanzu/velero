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
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
)

// RepositoryEnsurer ensures that backup repositories are created and ready.
type RepositoryEnsurer struct {
	log        logrus.FieldLogger
	repoLister velerov1listers.BackupRepositoryLister
	repoClient velerov1client.BackupRepositoriesGetter

	repoChansLock sync.Mutex
	repoChans     map[string]chan *velerov1api.BackupRepository

	// repoLocksMu synchronizes reads/writes to the repoLocks map itself
	// since maps are not threadsafe.
	repoLocksMu sync.Mutex
	repoLocks   map[repoKey]*sync.Mutex
}

type repoKey struct {
	volumeNamespace string
	backupLocation  string
	repositoryType  string
}

func NewRepositoryEnsurer(repoInformer velerov1informers.BackupRepositoryInformer, repoClient velerov1client.BackupRepositoriesGetter, log logrus.FieldLogger) *RepositoryEnsurer {
	r := &RepositoryEnsurer{
		log:        log,
		repoLister: repoInformer.Lister(),
		repoClient: repoClient,
		repoChans:  make(map[string]chan *velerov1api.BackupRepository),
		repoLocks:  make(map[repoKey]*sync.Mutex),
	}

	repoInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, upd interface{}) {
				oldObj := old.(*velerov1api.BackupRepository)
				newObj := upd.(*velerov1api.BackupRepository)

				// we're only interested in phase-changing updates
				if oldObj.Status.Phase == newObj.Status.Phase {
					return
				}

				// we're only interested in updates where the updated object is either Ready or NotReady
				if newObj.Status.Phase != velerov1api.BackupRepositoryPhaseReady && newObj.Status.Phase != velerov1api.BackupRepositoryPhaseNotReady {
					return
				}

				r.repoChansLock.Lock()
				defer r.repoChansLock.Unlock()

				key := repoLabels(newObj.Spec.VolumeNamespace, newObj.Spec.BackupStorageLocation, newObj.Spec.RepositoryType).String()
				repoChan, ok := r.repoChans[key]
				if !ok {
					log.Debugf("No ready channel found for repository %s/%s", newObj.Namespace, newObj.Name)
					return
				}

				repoChan <- newObj
			},
		},
	)

	return r
}

func repoLabels(volumeNamespace, backupLocation, repositoryType string) labels.Set {
	return map[string]string{
		velerov1api.VolumeNamespaceLabel: label.GetValidName(volumeNamespace),
		velerov1api.StorageLocationLabel: label.GetValidName(backupLocation),
		velerov1api.RepositoryTypeLabel:  label.GetValidName(repositoryType),
	}
}

func (r *RepositoryEnsurer) EnsureRepo(ctx context.Context, namespace, volumeNamespace, backupLocation, repositoryType string) (*velerov1api.BackupRepository, error) {
	if volumeNamespace == "" || backupLocation == "" || repositoryType == "" {
		return nil, errors.Errorf("wrong parameters, namespace %q, backup storage location %q, repository type %q", volumeNamespace, backupLocation, repositoryType)
	}

	log := r.log.WithField("volumeNamespace", volumeNamespace).WithField("backupLocation", backupLocation).WithField("repositoryType", repositoryType)

	// It's only safe to have one instance of this method executing concurrently for a
	// given volumeNamespace + backupLocation + repositoryType, so synchronize based on that. It's fine
	// to run concurrently for *different* namespaces/locations. If you had 2 goroutines
	// running this for the same inputs, both might find no BackupRepository exists, then
	// both would create new ones for the same namespace/location.
	//
	// This issue could probably be avoided if we had a deterministic name for
	// each restic repository, and we just tried to create it, checked for an
	// AlreadyExists err, and then waited for it to be ready. However, there are
	// already repositories in the wild with non-deterministic names (i.e. using
	// GenerateName) which poses a backwards compatibility problem.
	log.Debug("Acquiring lock")

	repoMu := r.repoLock(volumeNamespace, backupLocation, repositoryType)
	repoMu.Lock()
	defer func() {
		repoMu.Unlock()
		log.Debug("Released lock")
	}()

	log.Debug("Acquired lock")

	selector := labels.SelectorFromSet(repoLabels(volumeNamespace, backupLocation, repositoryType))

	repos, err := r.repoLister.BackupRepositories(namespace).List(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(repos) > 1 {
		return nil, errors.Errorf("more than one BackupRepository found for workload namespace %q, backup storage location %q, repository type %q", volumeNamespace, backupLocation, repositoryType)
	}
	if len(repos) == 1 {
		if repos[0].Status.Phase != velerov1api.BackupRepositoryPhaseReady {
			return nil, errors.Errorf("restic repository is not ready: %s", repos[0].Status.Message)
		}

		log.Debug("Ready repository found")
		return repos[0], nil
	}

	log.Debug("No repository found, creating one")

	// no repo found: create one and wait for it to be ready
	repo := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: fmt.Sprintf("%s-%s-%s-", volumeNamespace, backupLocation, repositoryType),
			Labels:       repoLabels(volumeNamespace, backupLocation, repositoryType),
		},
		Spec: velerov1api.BackupRepositorySpec{
			VolumeNamespace:       volumeNamespace,
			BackupStorageLocation: backupLocation,
			RepositoryType:        repositoryType,
		},
	}

	repoChan := r.getRepoChan(selector.String())
	defer func() {
		delete(r.repoChans, selector.String())
		close(repoChan)
	}()

	if _, err := r.repoClient.BackupRepositories(namespace).Create(context.TODO(), repo, metav1.CreateOptions{}); err != nil {
		return nil, errors.Wrapf(err, "unable to create restic repository resource")
	}

	select {
	// repositories should become either ready or not ready quickly if they're
	// newly created.
	case <-time.After(time.Minute):
		return nil, errors.New("timed out waiting for restic repository to become ready")
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for restic repository to become ready")
	case res := <-repoChan:

		if res.Status.Phase == velerov1api.BackupRepositoryPhaseNotReady {
			return nil, errors.Errorf("restic repository is not ready: %s", res.Status.Message)
		}

		return res, nil
	}
}

func (r *RepositoryEnsurer) getRepoChan(name string) chan *velerov1api.BackupRepository {
	r.repoChansLock.Lock()
	defer r.repoChansLock.Unlock()

	r.repoChans[name] = make(chan *velerov1api.BackupRepository)
	return r.repoChans[name]
}

func (r *RepositoryEnsurer) repoLock(volumeNamespace, backupLocation, repositoryType string) *sync.Mutex {
	r.repoLocksMu.Lock()
	defer r.repoLocksMu.Unlock()

	key := repoKey{
		volumeNamespace: volumeNamespace,
		backupLocation:  backupLocation,
		repositoryType:  repositoryType,
	}

	if r.repoLocks[key] == nil {
		r.repoLocks[key] = new(sync.Mutex)
	}

	return r.repoLocks[key]
}
