/*
Copyright 2018 the Velero contributors.

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

package podvolume

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	"github.com/vmware-tanzu/velero/pkg/repository"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

type RestoreData struct {
	Restore                         *velerov1api.Restore
	Pod                             *corev1api.Pod
	PodVolumeBackups                []*velerov1api.PodVolumeBackup
	SourceNamespace, BackupLocation string
}

// Restorer can execute pod volume restores of volumes in a pod.
type Restorer interface {
	// RestorePodVolumes restores all annotated volumes in a pod.
	RestorePodVolumes(RestoreData) []error
}

type restorer struct {
	ctx          context.Context
	repoLocker   *repository.RepoLocker
	repoEnsurer  *repository.RepositoryEnsurer
	veleroClient clientset.Interface
	pvcClient    corev1client.PersistentVolumeClaimsGetter
	podClient    corev1client.PodsGetter
	kubeClient   kubernetes.Interface

	resultsLock    sync.Mutex
	results        map[string]chan *velerov1api.PodVolumeRestore
	nodeAgentCheck chan error
	log            logrus.FieldLogger
}

func newRestorer(
	ctx context.Context,
	repoLocker *repository.RepoLocker,
	repoEnsurer *repository.RepositoryEnsurer,
	podVolumeRestoreInformer cache.SharedIndexInformer,
	veleroClient clientset.Interface,
	pvcClient corev1client.PersistentVolumeClaimsGetter,
	podClient corev1client.PodsGetter,
	kubeClient kubernetes.Interface,
	log logrus.FieldLogger,
) *restorer {
	r := &restorer{
		ctx:          ctx,
		repoLocker:   repoLocker,
		repoEnsurer:  repoEnsurer,
		veleroClient: veleroClient,
		pvcClient:    pvcClient,
		podClient:    podClient,
		kubeClient:   kubeClient,

		results: make(map[string]chan *velerov1api.PodVolumeRestore),
		log:     log,
	}

	podVolumeRestoreInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, obj interface{}) {
				pvr := obj.(*velerov1api.PodVolumeRestore)

				if pvr.Status.Phase == velerov1api.PodVolumeRestorePhaseCompleted || pvr.Status.Phase == velerov1api.PodVolumeRestorePhaseFailed {
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

func (r *restorer) RestorePodVolumes(data RestoreData) []error {
	volumesToRestore := getVolumeBackupInfoForPod(data.PodVolumeBackups, data.Pod, data.SourceNamespace)
	if len(volumesToRestore) == 0 {
		return nil
	}

	if err := nodeagent.IsRunning(r.ctx, r.kubeClient, data.Restore.Namespace); err != nil {
		return []error{errors.Wrapf(err, "error to check node agent status")}
	}

	repositoryType, err := getVolumesRepositoryType(volumesToRestore)
	if err != nil {
		return []error{err}
	}

	repo, err := r.repoEnsurer.EnsureRepo(r.ctx, data.Restore.Namespace, data.SourceNamespace, data.BackupLocation, repositoryType)
	if err != nil {
		return []error{err}
	}

	// get a single non-exclusive lock since we'll wait for all individual
	// restores to be complete before releasing it.
	r.repoLocker.Lock(repo.Name)
	defer r.repoLocker.Unlock(repo.Name)

	resultsChan := make(chan *velerov1api.PodVolumeRestore)

	r.resultsLock.Lock()
	r.results[resultsKey(data.Pod.Namespace, data.Pod.Name)] = resultsChan
	r.resultsLock.Unlock()

	r.nodeAgentCheck = make(chan error)

	var (
		errs        []error
		numRestores int
		podVolumes  = make(map[string]corev1api.Volume)
	)

	// put the pod's volumes in a map for efficient lookup below
	for _, podVolume := range data.Pod.Spec.Volumes {
		podVolumes[podVolume.Name] = podVolume
	}
	for volume, backupInfo := range volumesToRestore {
		volumeObj, ok := podVolumes[volume]
		var pvc *corev1api.PersistentVolumeClaim
		if ok {
			if volumeObj.PersistentVolumeClaim != nil {
				pvc, err = r.pvcClient.PersistentVolumeClaims(data.Pod.Namespace).Get(context.TODO(), volumeObj.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
				if err != nil {
					errs = append(errs, errors.Wrap(err, "error getting persistent volume claim for volume"))
					continue
				}
			}
		}

		volumeRestore := newPodVolumeRestore(data.Restore, data.Pod, data.BackupLocation, volume, backupInfo.snapshotID, repo.Spec.ResticIdentifier, backupInfo.uploaderType, data.SourceNamespace, pvc)

		if err := errorOnly(r.veleroClient.VeleroV1().PodVolumeRestores(volumeRestore.Namespace).Create(context.TODO(), volumeRestore, metav1.CreateOptions{})); err != nil {
			errs = append(errs, errors.WithStack(err))
			continue
		}
		numRestores++
	}

	checkCtx, checkCancel := context.WithCancel(context.Background())
	go func() {
		nodeName := ""

		checkFunc := func(ctx context.Context) (bool, error) {
			newObj, err := r.kubeClient.CoreV1().Pods(data.Pod.Namespace).Get(ctx, data.Pod.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			nodeName = newObj.Spec.NodeName

			err = kube.IsPodScheduled(newObj)
			if err != nil {
				return false, nil
			} else {
				return true, nil
			}
		}

		err := wait.PollWithContext(checkCtx, time.Millisecond*500, time.Minute*10, checkFunc)
		if err == wait.ErrWaitTimeout {
			r.log.WithError(err).Error("Restoring pod is not scheduled until timeout or cancel, disengage")
		} else if err != nil {
			r.log.WithError(err).Error("Failed to check node-agent pod status, disengage")
		} else {
			err = nodeagent.IsRunningInNode(checkCtx, data.Restore.Namespace, nodeName, r.podClient)
			if err != nil {
				r.log.WithField("node", nodeName).WithError(err).Error("node-agent pod is not running in node, abort the restore")
				r.nodeAgentCheck <- errors.Wrapf(err, "node-agent pod is not running in node %s", nodeName)
			}
		}
	}()

ForEachVolume:
	for i := 0; i < numRestores; i++ {
		select {
		case <-r.ctx.Done():
			errs = append(errs, errors.New("timed out waiting for all PodVolumeRestores to complete"))
			break ForEachVolume
		case res := <-resultsChan:
			if res.Status.Phase == velerov1api.PodVolumeRestorePhaseFailed {
				errs = append(errs, errors.Errorf("pod volume restore failed: %s", res.Status.Message))
			}
		case err := <-r.nodeAgentCheck:
			errs = append(errs, err)
			break ForEachVolume
		}
	}

	// This is to prevent the case that resultsChan is signaled before nodeAgentCheck though this is unlikely possible.
	// One possible case is that the CR is edited and set to an ending state manually, either completed or failed.
	// In this case, we must notify the check routine to stop.
	checkCancel()

	r.resultsLock.Lock()
	delete(r.results, resultsKey(data.Pod.Namespace, data.Pod.Name))
	r.resultsLock.Unlock()

	return errs
}

func newPodVolumeRestore(restore *velerov1api.Restore, pod *corev1api.Pod, backupLocation, volume, snapshot, repoIdentifier, uploaderType, sourceNamespace string, pvc *corev1api.PersistentVolumeClaim) *velerov1api.PodVolumeRestore {
	pvr := &velerov1api.PodVolumeRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    restore.Namespace,
			GenerateName: restore.Name + "-",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: velerov1api.SchemeGroupVersion.String(),
					Kind:       "Restore",
					Name:       restore.Name,
					UID:        restore.UID,
					Controller: boolptr.True(),
				},
			},
			Labels: map[string]string{
				velerov1api.RestoreNameLabel: label.GetValidName(restore.Name),
				velerov1api.RestoreUIDLabel:  string(restore.UID),
				velerov1api.PodUIDLabel:      string(pod.UID),
			},
		},
		Spec: velerov1api.PodVolumeRestoreSpec{
			Pod: corev1api.ObjectReference{
				Kind:      "Pod",
				Namespace: pod.Namespace,
				Name:      pod.Name,
				UID:       pod.UID,
			},
			Volume:                volume,
			SnapshotID:            snapshot,
			BackupStorageLocation: backupLocation,
			RepoIdentifier:        repoIdentifier,
			UploaderType:          uploaderType,
			SourceNamespace:       sourceNamespace,
		},
	}
	if pvc != nil {
		// this label is not used by velero, but useful for debugging.
		pvr.Labels[velerov1api.PVCUIDLabel] = string(pvc.UID)
	}
	return pvr
}

func getVolumesRepositoryType(volumes map[string]volumeBackupInfo) (string, error) {
	if len(volumes) == 0 {
		return "", errors.New("empty volume list")
	}

	// the podVolumeBackups list come from one backup. In one backup, it is impossible that volumes are
	// backed up by different uploaders or to different repositories. Asserting this ensures one repo only,
	// which will simplify the following logics
	repositoryType := ""
	for _, backupInfo := range volumes {
		if backupInfo.repositoryType == "" {
			return "", errors.Errorf("empty repository type found among volume snapshots, snapshot ID %s, uploader %s",
				backupInfo.snapshotID, backupInfo.uploaderType)
		}

		if repositoryType == "" {
			repositoryType = backupInfo.repositoryType
		} else if repositoryType != backupInfo.repositoryType {
			return "", errors.Errorf("multiple repository type in one backup, current type %s, differential one [type %s, snapshot ID %s, uploader %s]",
				repositoryType, backupInfo.repositoryType, backupInfo.snapshotID, backupInfo.uploaderType)
		}
	}

	return repositoryType, nil
}
