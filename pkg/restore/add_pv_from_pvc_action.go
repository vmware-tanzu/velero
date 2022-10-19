/*
Copyright 2019 the Velero contributors.

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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type AddPVFromPVCAction struct {
	logger logrus.FieldLogger
}

func NewAddPVFromPVCAction(logger logrus.FieldLogger) *AddPVFromPVCAction {
	return &AddPVFromPVCAction{logger: logger}
}

func (a *AddPVFromPVCAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

func (a *AddPVFromPVCAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	a.logger.Info("Executing AddPVFromPVCAction")

	// use input.ItemFromBackup because we need to look at status fields, which have already been
	// removed from input.Item
	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to persistent volume claim")
	}

	// TODO: consolidate this logic in a helper function to share with backup_pv_action.go
	if pvc.Status.Phase != corev1api.ClaimBound || pvc.Spec.VolumeName == "" {
		a.logger.Info("PVC is not bound or its volume name is empty")
		return &velero.RestoreItemActionExecuteOutput{
			UpdatedItem: input.Item,
		}, nil
	}

	pv := velero.ResourceIdentifier{
		GroupResource: kuberesource.PersistentVolumes,
		Name:          pvc.Spec.VolumeName,
	}

	a.logger.Infof("Adding PV %s as an additional item to restore", pvc.Spec.VolumeName)
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     input.Item,
		AdditionalItems: []velero.ResourceIdentifier{pv},
	}, nil
}
