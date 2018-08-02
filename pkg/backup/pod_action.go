/*
Copyright 2017 Heptio Inc.

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
	"github.com/heptio/ark/pkg/kuberesource"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

// podAction implements ItemAction.
type podAction struct {
	log logrus.FieldLogger
}

// NewPodAction creates a new ItemAction for pods.
func NewPodAction(logger logrus.FieldLogger) ItemAction {
	return &podAction{log: logger}
}

// AppliesTo returns a ResourceSelector that applies only to pods.
func (a *podAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

// Execute scans the pod's spec.volumes for persistentVolumeClaim volumes and returns a
// ResourceIdentifier list containing references to all of the persistentVolumeClaim volumes used by
// the pod. This ensures that when a pod is backed up, all referenced PVCs are backed up too.
func (a *podAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []ResourceIdentifier, error) {
	a.log.Info("Executing podAction")
	defer a.log.Info("Done executing podAction")

	pod := new(corev1api.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), pod); err != nil {
		return item, nil, errors.WithStack(err)
	}

	if len(pod.Spec.Volumes) == 0 {
		a.log.Info("pod has no volumes")
		return item, nil, nil
	}

	var pvcs []ResourceIdentifier
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		a.log.Infof("Adding pvc %s to additionalItems", volume.PersistentVolumeClaim.ClaimName)
		pvcs = append(pvcs, ResourceIdentifier{
			GroupResource: kuberesource.PersistentVolumeClaims,
			Namespace:     pod.Namespace,
			Name:          volume.PersistentVolumeClaim.ClaimName,
		})
	}

	return item, pvcs, nil
}
