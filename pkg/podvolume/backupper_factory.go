/*
Copyright the Velero contributors.

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
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository"
)

// BackupperFactory can construct pod volumes backuppers.
type BackupperFactory interface {
	// NewBackupper returns a pod volumes backupper for use during a single Velero backup.
	NewBackupper(context.Context, *velerov1api.Backup, string) (Backupper, error)
}

func NewBackupperFactory(
	repoLocker *repository.RepoLocker,
	repoEnsurer *repository.RepositoryEnsurer,
	veleroClient clientset.Interface,
	pvcClient corev1client.PersistentVolumeClaimsGetter,
	pvClient corev1client.PersistentVolumesGetter,
	podClient corev1client.PodsGetter,
	log logrus.FieldLogger,
) BackupperFactory {
	return &backupperFactory{
		repoLocker:   repoLocker,
		repoEnsurer:  repoEnsurer,
		veleroClient: veleroClient,
		pvcClient:    pvcClient,
		pvClient:     pvClient,
		podClient:    podClient,
		log:          log,
	}
}

type backupperFactory struct {
	repoLocker   *repository.RepoLocker
	repoEnsurer  *repository.RepositoryEnsurer
	veleroClient clientset.Interface
	pvcClient    corev1client.PersistentVolumeClaimsGetter
	pvClient     corev1client.PersistentVolumesGetter
	podClient    corev1client.PodsGetter
	log          logrus.FieldLogger
}

func (bf *backupperFactory) NewBackupper(ctx context.Context, backup *velerov1api.Backup, uploaderType string) (Backupper, error) {
	informer := velerov1informers.NewFilteredPodVolumeBackupInformer(
		bf.veleroClient,
		backup.Namespace,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf("%s=%s", velerov1api.BackupUIDLabel, backup.UID)
		},
	)

	b := newBackupper(ctx, bf.repoLocker, bf.repoEnsurer, informer, bf.veleroClient, bf.pvcClient, bf.pvClient, bf.podClient, uploaderType, bf.log)

	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, errors.New("timed out waiting for caches to sync")
	}

	return b, nil
}
