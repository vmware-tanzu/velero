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

package restore

import (
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/kuberesource"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type addPVCFromPodAction struct {
	logger logrus.FieldLogger
}

func NewAddPVCFromPodAction(logger logrus.FieldLogger) ItemAction {
	return &addPVCFromPodAction{logger: logger}
}

func (a *addPVCFromPodAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *addPVCFromPodAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, []ResourceIdentifier, error, error) {
	a.logger.Info("Executing addPVCFromPodAction")

	var pod corev1api.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pod); err != nil {
		return nil, nil, nil, errors.Wrap(err, "unable to convert unstructured item to pod")
	}

	var additionalItems []ResourceIdentifier

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		a.logger.Infof("Found a PVC we need to back up: %s/%s", pod.Namespace, volume.PersistentVolumeClaim.ClaimName)
		additionalItems = append(additionalItems, ResourceIdentifier{
			GroupResource: kuberesource.PersistentVolumeClaims,
			Namespace:     pod.Namespace,
			Name:          volume.PersistentVolumeClaim.ClaimName,
		})
	}

	return obj, additionalItems, nil, nil
}
