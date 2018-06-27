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
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/boolptr"
)

// Restorer can execute restic restores of volumes in a pod.
type Restorer interface {
	// RestorePodVolumes restores all annotated volumes in a pod.
	RestorePodVolumes(restore *arkv1api.Restore, pod *corev1api.Pod, log logrus.FieldLogger) []error
}

type restorer struct {
	ctx         context.Context
	repoManager *repositoryManager
	repoEnsurer *repositoryEnsurer

	resultsLock sync.Mutex
	results     map[string]chan *arkv1api.PodVolumeRestore
}

func newRestorer(
	ctx context.Context,
	rm *repositoryManager,
	repoEnsurer *repositoryEnsurer,
	podVolumeRestoreInformer cache.SharedIndexInformer,
	log logrus.FieldLogger,
) *restorer {
	r := &restorer{
		ctx:         ctx,
		repoManager: rm,
		repoEnsurer: repoEnsurer,

		results: make(map[string]chan *arkv1api.PodVolumeRestore),
	}

	podVolumeRestoreInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, obj interface{}) {
				pvr := obj.(*arkv1api.PodVolumeRestore)

				if pvr.Status.Phase == arkv1api.PodVolumeRestorePhaseCompleted || pvr.Status.Phase == arkv1api.PodVolumeRestorePhaseFailed {
					r.resultsLock.Lock()
					defer r.resultsLock.Unlock()

					resChan, ok := r.results[resultsKey(pvr.Spec.Pod.Namespace, pvr.Spec.Pod.Name)]
					if !ok {
						log.Errorf("No results channel found for pod %s/%s to send pod volume restore %s/%s on", pvr.Spec.Pod.Namespace, pvr.Spec.Pod.Name, pvr.Namespace, pvr.Name)
						return
					}
					resChan <- pvr
				}
			},
		},
	)

	return r
}

func (r *restorer) RestorePodVolumes(restore *arkv1api.Restore, pod *corev1api.Pod, log logrus.FieldLogger) []error {
	// get volumes to restore from pod's annotations
	volumesToRestore := GetPodSnapshotAnnotations(pod)
	if len(volumesToRestore) == 0 {
		return nil
	}

	repo, err := r.repoEnsurer.EnsureRepo(r.ctx, restore.Namespace, pod.Namespace)
	if err != nil {
		return []error{err}
	}

	// get a single non-exclusive lock since we'll wait for all individual
	// restores to be complete before releasing it.
	r.repoManager.repoLocker.Lock(pod.Namespace)
	defer r.repoManager.repoLocker.Unlock(pod.Namespace)

	resultsChan := make(chan *arkv1api.PodVolumeRestore)

	r.resultsLock.Lock()
	r.results[resultsKey(pod.Namespace, pod.Name)] = resultsChan
	r.resultsLock.Unlock()

	var (
		errs        []error
		numRestores int
	)

	for volume, snapshot := range volumesToRestore {
		volumeRestore := newPodVolumeRestore(restore, pod, volume, snapshot, repo.Spec.ResticIdentifier)

		if err := errorOnly(r.repoManager.arkClient.ArkV1().PodVolumeRestores(volumeRestore.Namespace).Create(volumeRestore)); err != nil {
			errs = append(errs, errors.WithStack(err))
			continue
		}
		numRestores++
	}

ForEachVolume:
	for i := 0; i < numRestores; i++ {
		select {
		case <-r.ctx.Done():
			errs = append(errs, errors.New("timed out waiting for all PodVolumeRestores to complete"))
			break ForEachVolume
		case res := <-resultsChan:
			if res.Status.Phase == arkv1api.PodVolumeRestorePhaseFailed {
				errs = append(errs, errors.Errorf("pod volume restore failed: %s", res.Status.Message))
			}
		}
	}

	r.resultsLock.Lock()
	delete(r.results, resultsKey(pod.Namespace, pod.Name))
	r.resultsLock.Unlock()

	return errs
}

func newPodVolumeRestore(restore *arkv1api.Restore, pod *corev1api.Pod, volume, snapshot, repoIdentifier string) *arkv1api.PodVolumeRestore {
	return &arkv1api.PodVolumeRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    restore.Namespace,
			GenerateName: restore.Name + "-",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: arkv1api.SchemeGroupVersion.String(),
					Kind:       "Restore",
					Name:       restore.Name,
					UID:        restore.UID,
					Controller: boolptr.True(),
				},
			},
			Labels: map[string]string{
				arkv1api.RestoreNameLabel: restore.Name,
				arkv1api.RestoreUIDLabel:  string(restore.UID),
				arkv1api.PodUIDLabel:      string(pod.UID),
			},
		},
		Spec: arkv1api.PodVolumeRestoreSpec{
			Pod: corev1api.ObjectReference{
				Kind:      "Pod",
				Namespace: pod.Namespace,
				Name:      pod.Name,
				UID:       pod.UID,
			},
			Volume:         volume,
			SnapshotID:     snapshot,
			RepoIdentifier: repoIdentifier,
		},
	}
}
