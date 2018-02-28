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

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/boolptr"
)

// Backupper can execute restic backups of volumes in a pod.
type Backupper interface {
	// BackupPodVolumes backs up all annotated volumes in a pod.
	BackupPodVolumes(backup *arkv1api.Backup, pod *corev1api.Pod, log logrus.FieldLogger) (map[string]string, []error)
}

type backupper struct {
	repoManager *repositoryManager
	ctx         context.Context

	results     map[string]chan *arkv1api.PodVolumeBackup
	resultsLock sync.Mutex
}

func newBackupper(ctx context.Context, repoManager *repositoryManager, podVolumeBackupInformer cache.SharedIndexInformer) *backupper {
	b := &backupper{
		repoManager: repoManager,
		ctx:         ctx,
		results:     make(map[string]chan *arkv1api.PodVolumeBackup),
	}

	podVolumeBackupInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, obj interface{}) {
				pvb := obj.(*arkv1api.PodVolumeBackup)

				if pvb.Status.Phase == arkv1api.PodVolumeBackupPhaseCompleted || pvb.Status.Phase == arkv1api.PodVolumeBackupPhaseFailed {
					b.resultsLock.Lock()
					b.results[resultsKey(pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name)] <- pvb
					b.resultsLock.Unlock()
				}
			},
		},
	)

	return b
}

func resultsKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}

func (b *backupper) BackupPodVolumes(backup *arkv1api.Backup, pod *corev1api.Pod, log logrus.FieldLogger) (map[string]string, []error) {
	// get volumes to backup from pod's annotations
	volumesToBackup := GetVolumesToBackup(pod)
	if len(volumesToBackup) == 0 {
		return nil, nil
	}

	// ensure a repo exists for the pod's namespace
	if err := b.repoManager.ensureRepo(pod.Namespace); err != nil {
		return nil, []error{err}
	}

	resultsChan := make(chan *arkv1api.PodVolumeBackup)

	b.resultsLock.Lock()
	b.results[resultsKey(pod.Namespace, pod.Name)] = resultsChan
	b.resultsLock.Unlock()

	var (
		errs            []error
		volumeSnapshots = make(map[string]string)
	)

	for _, volumeName := range volumesToBackup {
		b.repoManager.repoLocker.Lock(pod.Namespace)
		defer b.repoManager.repoLocker.Unlock(pod.Namespace)

		volumeBackup := newPodVolumeBackup(backup, pod, volumeName, b.repoManager.config.repoPrefix)

		if err := errorOnly(b.repoManager.arkClient.ArkV1().PodVolumeBackups(volumeBackup.Namespace).Create(volumeBackup)); err != nil {
			errs = append(errs, err)
			continue
		}

		volumeSnapshots[volumeName] = ""
	}

ForEachVolume:
	for i, count := 0, len(volumeSnapshots); i < count; i++ {
		select {
		case <-b.ctx.Done():
			errs = append(errs, errors.New("timed out waiting for all PodVolumeBackups to complete"))
			break ForEachVolume
		case res := <-resultsChan:
			switch res.Status.Phase {
			case arkv1api.PodVolumeBackupPhaseCompleted:
				volumeSnapshots[res.Spec.Volume] = res.Status.SnapshotID
			case arkv1api.PodVolumeBackupPhaseFailed:
				errs = append(errs, errors.Errorf("pod volume backup failed: %s", res.Status.Message))
				delete(volumeSnapshots, res.Spec.Volume)
			}
		}
	}

	b.resultsLock.Lock()
	delete(b.results, resultsKey(pod.Namespace, pod.Name))
	b.resultsLock.Unlock()

	return volumeSnapshots, errs
}

func newPodVolumeBackup(backup *arkv1api.Backup, pod *corev1api.Pod, volumeName, repoPrefix string) *arkv1api.PodVolumeBackup {
	return &arkv1api.PodVolumeBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    backup.Namespace,
			GenerateName: backup.Name + "-",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: arkv1api.SchemeGroupVersion.String(),
					Kind:       "Backup",
					Name:       backup.Name,
					UID:        backup.UID,
					Controller: boolptr.True(),
				},
			},
			Labels: map[string]string{
				arkv1api.BackupNameLabel: backup.Name,
				arkv1api.BackupUIDLabel:  string(backup.UID),
			},
		},
		Spec: arkv1api.PodVolumeBackupSpec{
			Node: pod.Spec.NodeName,
			Pod: corev1api.ObjectReference{
				Kind:      "Pod",
				Namespace: pod.Namespace,
				Name:      pod.Name,
				UID:       pod.UID,
			},
			Volume: volumeName,
			Tags: map[string]string{
				"backup":     backup.Name,
				"backup-uid": string(backup.UID),
				"pod":        pod.Name,
				"pod-uid":    string(pod.UID),
				"ns":         pod.Namespace,
				"volume":     volumeName,
			},
			RepoPrefix: repoPrefix,
		},
	}
}
