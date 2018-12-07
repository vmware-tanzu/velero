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

type addPVFromPVCAction struct {
	logger logrus.FieldLogger
}

func NewAddPVFromPVCAction(logger logrus.FieldLogger) ItemAction {
	return &addPVFromPVCAction{logger: logger}
}

func (a *addPVFromPVCAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

func (a *addPVFromPVCAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, []ResourceIdentifier, error, error) {
	a.logger.Info("Executing addPVFromPVCAction")

	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pvc); err != nil {
		return nil, nil, nil, errors.Wrap(err, "unable to convert unstructured item to persistent volume claim")
	}

	// TODO: consolidate this logic in a helper function to share with backup_pv_action.go
	// FIXME: we need to check the PVC from the backup, because obj has already had its status removed.
	// Depends on https://github.com/heptio/ark/pull/1123
	if pvc.Status.Phase != corev1api.ClaimBound || pvc.Spec.VolumeName == "" {
		a.logger.Info("PVC is not bound or its volume name is empty")
		return obj, nil, nil, nil
	}

	pv := ResourceIdentifier{
		GroupResource: kuberesource.PersistentVolumes,
		Name:          pvc.Spec.VolumeName,
	}

	a.logger.Infof("Adding PV %s as an additional item to restore", pvc.Spec.VolumeName)
	return obj, []ResourceIdentifier{pv}, nil, nil
}
