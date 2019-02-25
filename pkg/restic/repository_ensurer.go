/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	arkv1informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	arkv1listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
)

// repositoryEnsurer ensures that Ark restic repositories are created and ready.
type repositoryEnsurer struct {
	log        logrus.FieldLogger
	repoLister arkv1listers.ResticRepositoryLister
	repoClient arkv1client.ResticRepositoriesGetter

	readyChansLock sync.Mutex
	readyChans     map[string]chan *arkv1api.ResticRepository

	// repoLocksMu synchronizes reads/writes to the repoLocks map itself
	// since maps are not threadsafe.
	repoLocksMu sync.Mutex
	repoLocks   map[repoKey]*sync.Mutex
}

type repoKey struct {
	volumeNamespace string
	backupLocation  string
}

func newRepositoryEnsurer(repoInformer arkv1informers.ResticRepositoryInformer, repoClient arkv1client.ResticRepositoriesGetter, log logrus.FieldLogger) *repositoryEnsurer {
	r := &repositoryEnsurer{
		log:        log,
		repoLister: repoInformer.Lister(),
		repoClient: repoClient,
		readyChans: make(map[string]chan *arkv1api.ResticRepository),
		repoLocks:  make(map[repoKey]*sync.Mutex),
	}

	repoInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, upd interface{}) {
				oldObj := old.(*arkv1api.ResticRepository)
				newObj := upd.(*arkv1api.ResticRepository)

				if oldObj.Status.Phase != arkv1api.ResticRepositoryPhaseReady && newObj.Status.Phase == arkv1api.ResticRepositoryPhaseReady {
					r.readyChansLock.Lock()
					defer r.readyChansLock.Unlock()

					key := repoLabels(newObj.Spec.VolumeNamespace, newObj.Spec.BackupStorageLocation).String()
					readyChan, ok := r.readyChans[key]
					if !ok {
						log.Errorf("No ready channel found for repository %s/%s", newObj.Namespace, newObj.Name)
						return
					}

					readyChan <- newObj
					delete(r.readyChans, key)
				}
			},
		},
	)

	return r
}

func repoLabels(volumeNamespace, backupLocation string) labels.Set {
	return map[string]string{
		arkv1api.ResticVolumeNamespaceLabel: volumeNamespace,
		arkv1api.StorageLocationLabel:       backupLocation,
	}
}

func (r *repositoryEnsurer) EnsureRepo(ctx context.Context, namespace, volumeNamespace, backupLocation string) (*arkv1api.ResticRepository, error) {
	log := r.log.WithField("volumeNamespace", volumeNamespace).WithField("backupLocation", backupLocation)

	// It's only safe to have one instance of this method executing concurrently for a
	// given volumeNamespace + backupLocation, so synchronize based on that. It's fine
	// to run concurrently for *different* namespaces/locations. If you had 2 goroutines
	// running this for the same inputs, both might find no ResticRepository exists, then
	// both would create new ones for the same namespace/location.
	//
	// This issue could probably be avoided if we had a deterministic name for
	// each restic repository, and we just tried to create it, checked for an
	// AlreadyExists err, and then waited for it to be ready. However, there are
	// already repositories in the wild with non-deterministic names (i.e. using
	// GenerateName) which poses a backwards compatibility problem.
	log.Debug("Acquiring lock")

	repoMu := r.repoLock(volumeNamespace, backupLocation)
	repoMu.Lock()
	defer func() {
		repoMu.Unlock()
		log.Debug("Released lock")
	}()

	log.Debug("Acquired lock")

	selector := labels.SelectorFromSet(repoLabels(volumeNamespace, backupLocation))

	repos, err := r.repoLister.ResticRepositories(namespace).List(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(repos) > 1 {
		return nil, errors.Errorf("more than one ResticRepository found for workload namespace %q, backup storage location %q", volumeNamespace, backupLocation)
	}
	if len(repos) == 1 {
		if repos[0].Status.Phase != arkv1api.ResticRepositoryPhaseReady {
			return nil, errors.New("restic repository is not ready")
		}

		log.Debug("Ready repository found")
		return repos[0], nil
	}

	log.Debug("No repository found, creating one")

	// no repo found: create one and wait for it to be ready
	repo := &arkv1api.ResticRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: fmt.Sprintf("%s-%s-", volumeNamespace, backupLocation),
			Labels:       repoLabels(volumeNamespace, backupLocation),
		},
		Spec: arkv1api.ResticRepositorySpec{
			VolumeNamespace:       volumeNamespace,
			BackupStorageLocation: backupLocation,
			MaintenanceFrequency:  metav1.Duration{Duration: DefaultMaintenanceFrequency},
		},
	}

	readyChan := r.getReadyChan(selector.String())
	defer close(readyChan)

	if _, err := r.repoClient.ResticRepositories(namespace).Create(repo); err != nil {
		return nil, errors.Wrapf(err, "unable to create restic repository resource")
	}

	select {
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for restic repository to become ready")
	case res := <-readyChan:
		return res, nil
	}
}

func (r *repositoryEnsurer) getReadyChan(name string) chan *arkv1api.ResticRepository {
	r.readyChansLock.Lock()
	defer r.readyChansLock.Unlock()

	r.readyChans[name] = make(chan *arkv1api.ResticRepository)
	return r.readyChans[name]
}

func (r *repositoryEnsurer) repoLock(volumeNamespace, backupLocation string) *sync.Mutex {
	r.repoLocksMu.Lock()
	defer r.repoLocksMu.Unlock()

	key := repoKey{
		volumeNamespace: volumeNamespace,
		backupLocation:  backupLocation,
	}

	if r.repoLocks[key] == nil {
		r.repoLocks[key] = new(sync.Mutex)
	}

	return r.repoLocks[key]
}
