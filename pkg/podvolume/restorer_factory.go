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
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository"
)

// RestorerFactory can construct pod volumes restorers.
type RestorerFactory interface {
	// NewRestorer returns a pod volumes restorer for use during a single Velero restore.
	NewRestorer(context.Context, *velerov1api.Restore) (Restorer, error)
}

func NewRestorerFactory(repoLocker *repository.RepoLocker,
	repoEnsurer *repository.RepositoryEnsurer,
	veleroClient clientset.Interface,
	pvcClient corev1client.PersistentVolumeClaimsGetter,
	podClient corev1client.PodsGetter,
	kubeClient kubernetes.Interface,
	log logrus.FieldLogger) RestorerFactory {
	return &restorerFactory{
		repoLocker:   repoLocker,
		repoEnsurer:  repoEnsurer,
		veleroClient: veleroClient,
		pvcClient:    pvcClient,
		podClient:    podClient,
		kubeClient:   kubeClient,
		log:          log,
	}
}

type restorerFactory struct {
	repoLocker   *repository.RepoLocker
	repoEnsurer  *repository.RepositoryEnsurer
	veleroClient clientset.Interface
	pvcClient    corev1client.PersistentVolumeClaimsGetter
	podClient    corev1client.PodsGetter
	kubeClient   kubernetes.Interface
	log          logrus.FieldLogger
}

func (rf *restorerFactory) NewRestorer(ctx context.Context, restore *velerov1api.Restore) (Restorer, error) {
	informer := velerov1informers.NewFilteredPodVolumeRestoreInformer(
		rf.veleroClient,
		restore.Namespace,
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf("%s=%s", velerov1api.RestoreUIDLabel, restore.UID)
		},
	)

	r := newRestorer(ctx, rf.repoLocker, rf.repoEnsurer, informer, rf.veleroClient, rf.pvcClient, rf.podClient, rf.kubeClient, rf.log)

	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, errors.New("timed out waiting for cache to sync")
	}

	return r, nil
}
