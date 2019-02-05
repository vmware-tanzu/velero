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

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/heptio/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/heptio/velero/pkg/generated/listers/velero/v1"
)

// repositoryEnsurer ensures that Velero restic repositories are created and ready.
type repositoryEnsurer struct {
	repoLister velerov1listers.ResticRepositoryLister
	repoClient velerov1client.ResticRepositoriesGetter

	readyChansLock sync.Mutex
	readyChans     map[string]chan *velerov1api.ResticRepository
}

func newRepositoryEnsurer(repoInformer velerov1informers.ResticRepositoryInformer, repoClient velerov1client.ResticRepositoriesGetter, log logrus.FieldLogger) *repositoryEnsurer {
	r := &repositoryEnsurer{
		repoLister: repoInformer.Lister(),
		repoClient: repoClient,
		readyChans: make(map[string]chan *velerov1api.ResticRepository),
	}

	repoInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, upd interface{}) {
				oldObj := old.(*velerov1api.ResticRepository)
				newObj := upd.(*velerov1api.ResticRepository)

				if oldObj.Status.Phase != velerov1api.ResticRepositoryPhaseReady && newObj.Status.Phase == velerov1api.ResticRepositoryPhaseReady {
					r.readyChansLock.Lock()
					defer r.readyChansLock.Unlock()

					key := repoLabels(newObj.Spec.VolumeNamespace, newObj.Spec.BackupStorageLocation).String()
					readyChan, ok := r.readyChans[key]
					if !ok {
						log.Errorf("No ready channel found for repository %s/%s", newObj.Namespace, newObj.Name)
						return
					}

					readyChan <- newObj
					delete(r.readyChans, newObj.Name)
				}
			},
		},
	)

	return r
}

func repoLabels(volumeNamespace, backupLocation string) labels.Set {
	return map[string]string{
		velerov1api.ResticVolumeNamespaceLabel: volumeNamespace,
		velerov1api.StorageLocationLabel:       backupLocation,
	}
}

func (r *repositoryEnsurer) EnsureRepo(ctx context.Context, namespace, volumeNamespace, backupLocation string) (*velerov1api.ResticRepository, error) {
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
			return nil, errors.New("restic repository is not ready")
		}
		return repos[0], nil
	}

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

func (r *repositoryEnsurer) getReadyChan(name string) chan *velerov1api.ResticRepository {
	r.readyChansLock.Lock()
	defer r.readyChansLock.Unlock()

	r.readyChans[name] = make(chan *velerov1api.ResticRepository)
	return r.readyChans[name]
}
