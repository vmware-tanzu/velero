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

package restic

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

// repositoryEnsurer ensures that Velero restic repositories are created and ready.
type repositoryEnsurer struct {
	log        logrus.FieldLogger
	repoLister velerov1listers.ResticRepositoryLister
	repoClient velerov1client.ResticRepositoriesGetter

	repoChansLock sync.Mutex
	repoChans     map[string]chan *velerov1api.ResticRepository

	// repoLocksMu synchronizes reads/writes to the repoLocks map itself
	// since maps are not threadsafe.
	repoLocksMu sync.Mutex
	repoLocks   map[repoKey]*sync.Mutex
}

type repoKey struct {
	volumeNamespace string
	backupLocation  string
}

func newRepositoryEnsurer(repoInformer velerov1informers.ResticRepositoryInformer, repoClient velerov1client.ResticRepositoriesGetter, log logrus.FieldLogger) *repositoryEnsurer {
	r := &repositoryEnsurer{
		log:        log,
		repoLister: repoInformer.Lister(),
		repoClient: repoClient,
		repoChans:  make(map[string]chan *velerov1api.ResticRepository),
		repoLocks:  make(map[repoKey]*sync.Mutex),
	}

	repoInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, upd interface{}) {
				oldObj := old.(*velerov1api.ResticRepository)
				newObj := upd.(*velerov1api.ResticRepository)

				// we're only interested in phase-changing updates
				if oldObj.Status.Phase == newObj.Status.Phase {
					return
				}

				// we're only interested in updates where the updated object is either Ready or NotReady
				if newObj.Status.Phase != velerov1api.ResticRepositoryPhaseReady && newObj.Status.Phase != velerov1api.ResticRepositoryPhaseNotReady {
					return
				}

				r.repoChansLock.Lock()
				defer r.repoChansLock.Unlock()

				key := repoLabels(newObj.Spec.VolumeNamespace, newObj.Spec.BackupStorageLocation).String()
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

func repoLabels(volumeNamespace, backupLocation string) labels.Set {
	return map[string]string{
		velerov1api.ResticVolumeNamespaceLabel: label.GetValidName(volumeNamespace),
		velerov1api.StorageLocationLabel:       label.GetValidName(backupLocation),
	}
}

func (r *repositoryEnsurer) EnsureRepo(ctx context.Context, namespace, volumeNamespace, backupLocation string) (*velerov1api.ResticRepository, error) {
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
		if repos[0].Status.Phase != velerov1api.ResticRepositoryPhaseReady {
			return nil, errors.Errorf("restic repository is not ready: %s", repos[0].Status.Message)
		}

		log.Debug("Ready repository found")
		return repos[0], nil
	}

	log.Debug("No repository found, creating one")

	// no repo found: create one and wait for it to be ready
	repo := &velerov1api.ResticRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: fmt.Sprintf("%s-%s-", volumeNamespace, backupLocation),
			Labels:       repoLabels(volumeNamespace, backupLocation),
		},
		Spec: velerov1api.ResticRepositorySpec{
			VolumeNamespace:       volumeNamespace,
			BackupStorageLocation: backupLocation,
			MaintenanceFrequency:  metav1.Duration{Duration: DefaultMaintenanceFrequency},
		},
	}

	repoChan := r.getRepoChan(selector.String())
	defer func() {
		delete(r.repoChans, selector.String())
		close(repoChan)
	}()

	if _, err := r.repoClient.ResticRepositories(namespace).Create(context.TODO(), repo, metav1.CreateOptions{}); err != nil {
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
		if res.Status.Phase == velerov1api.ResticRepositoryPhaseNotReady {
			return nil, errors.Errorf("restic repository is not ready: %s", res.Status.Message)
		}

		return res, nil
	}
}

func (r *repositoryEnsurer) getRepoChan(name string) chan *velerov1api.ResticRepository {
	r.repoChansLock.Lock()
	defer r.repoChansLock.Unlock()

	r.repoChans[name] = make(chan *velerov1api.ResticRepository)
	return r.repoChans[name]
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
