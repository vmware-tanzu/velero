/*
Copyright 2017, 2019 the Velero contributors.

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

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PodAction implements ItemAction.
type PodAction struct {
	log       logrus.FieldLogger
	pvcClient corev1client.PersistentVolumeClaimsGetter
}

// NewPodAction creates a new ItemAction for pods.
func NewPodAction(logger logrus.FieldLogger, pvcClient corev1client.PersistentVolumeClaimsGetter) *PodAction {
	return &PodAction{
		log:       logger,
		pvcClient: pvcClient,
	}
}

// AppliesTo returns a ResourceSelector that applies only to pods.
func (a *PodAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

// Execute scans the pod's spec.volumes for persistentVolumeClaim volumes and returns a
// ResourceIdentifier list containing references to all of the persistentVolumeClaim volumes used by
// the pod that exist in the API. This ensures that when a pod is backed up, all referenced PVCs are backed up too.
func (a *PodAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.log.Info("Executing podAction")
	defer a.log.Info("Done executing podAction")

	pod := new(corev1api.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), pod); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	log := a.log.WithFields(logrus.Fields{
		"namespace": pod.Namespace,
		"name":      pod.Namespace,
	})

	if len(pod.Spec.Volumes) == 0 {
		log.Info("Pod has no volumes")
		return item, nil, nil
	}

	var additionalItems []velero.ResourceIdentifier
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName == "" {
			continue
		}

		pvcName := volume.PersistentVolumeClaim.ClaimName
		log.Infof("Pod has PVC volume %s", pvcName)

		_, err := a.pvcClient.PersistentVolumeClaims(pod.Namespace).Get(pvcName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			log.Infof("PVC %s does not exist, not adding to additionalItems", pvcName)
			continue
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "error checking if PVC exists")
		}

		log.Infof("Adding PVC %s to additionalItems", pvcName)
		additionalItems = append(additionalItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.PersistentVolumeClaims,
			Namespace:     pod.Namespace,
			Name:          pvcName,
		})
	}

	return item, additionalItems, nil
}
