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
	ctx         context.Context
	repoManager *repositoryManager
	repoEnsurer *repositoryEnsurer

	results     map[string]chan *arkv1api.PodVolumeBackup
	resultsLock sync.Mutex
}

func newBackupper(
	ctx context.Context,
	repoManager *repositoryManager,
	repoEnsurer *repositoryEnsurer,
	podVolumeBackupInformer cache.SharedIndexInformer,
	log logrus.FieldLogger,
) *backupper {
	b := &backupper{
		ctx:         ctx,
		repoManager: repoManager,
		repoEnsurer: repoEnsurer,

		results: make(map[string]chan *arkv1api.PodVolumeBackup),
	}

	podVolumeBackupInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, obj interface{}) {
				pvb := obj.(*arkv1api.PodVolumeBackup)

				if pvb.Status.Phase == arkv1api.PodVolumeBackupPhaseCompleted || pvb.Status.Phase == arkv1api.PodVolumeBackupPhaseFailed {
					b.resultsLock.Lock()
					defer b.resultsLock.Unlock()

					resChan, ok := b.results[resultsKey(pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name)]
					if !ok {
						log.Errorf("No results channel found for pod %s/%s to send pod volume backup %s/%s on", pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name, pvb.Namespace, pvb.Name)
						return
					}
					resChan <- pvb
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

	repo, err := b.repoEnsurer.EnsureRepo(b.ctx, backup.Namespace, pod.Namespace)
	if err != nil {
		return nil, []error{err}
	}

	// get a single non-exclusive lock since we'll wait for all individual
	// backups to be complete before releasing it.
	b.repoManager.repoLocker.Lock(pod.Namespace)
	defer b.repoManager.repoLocker.Unlock(pod.Namespace)

	resultsChan := make(chan *arkv1api.PodVolumeBackup)

	b.resultsLock.Lock()
	b.results[resultsKey(pod.Namespace, pod.Name)] = resultsChan
	b.resultsLock.Unlock()

	var (
		errs            []error
		volumeSnapshots = make(map[string]string)
		podVolumes      = make(map[string]corev1api.Volume)
	)

	// put the pod's volumes in a map for efficient lookup below
	for _, podVolume := range pod.Spec.Volumes {
		podVolumes[podVolume.Name] = podVolume
	}

	for _, volumeName := range volumesToBackup {
		if !volumeExists(podVolumes, volumeName) {
			log.Warnf("No volume named %s found in pod %s/%s, skipping", volumeName, pod.Namespace, pod.Name)
			continue
		}

		// hostPath volumes are not supported because they're not mounted into /var/lib/kubelet/pods, so our
		// daemonset pod has no way to access their data.
		if isHostPathVolume(podVolumes, volumeName) {
			log.Warnf("Volume %s in pod %s/%s is a hostPath volume which is not supported for restic backup, skipping", volumeName, pod.Namespace, pod.Name)
			continue
		}

		volumeBackup := newPodVolumeBackup(backup, pod, volumeName, repo.Spec.ResticIdentifier)

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

func volumeExists(podVolumes map[string]corev1api.Volume, volumeName string) bool {
	_, found := podVolumes[volumeName]
	return found
}

func isHostPathVolume(podVolumes map[string]corev1api.Volume, volumeName string) bool {
	volume, found := podVolumes[volumeName]
	if !found {
		return false
	}

	return volume.HostPath != nil
}

func newPodVolumeBackup(backup *arkv1api.Backup, pod *corev1api.Pod, volumeName, repoIdentifier string) *arkv1api.PodVolumeBackup {
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
			RepoIdentifier: repoIdentifier,
		},
	}
}

func errorOnly(_ interface{}, err error) error {
	return err
}
