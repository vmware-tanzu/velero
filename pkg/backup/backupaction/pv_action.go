/*
Copyright 2017 the Heptio Ark contributors.

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

package backupaction

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/kuberesource"
	"github.com/heptio/ark/pkg/plugin/interface/actioninterface"
)

// backupPVAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type backupPVAction struct {
	log logrus.FieldLogger
}

func NewBackupPVAction(logger logrus.FieldLogger) actioninterface.BackupItemAction {
	return &backupPVAction{log: logger}
}

func (a *backupPVAction) AppliesTo() (actioninterface.ResourceSelector, error) {
	return actioninterface.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// Execute finds the PersistentVolume bound by the provided
// PersistentVolumeClaim, if any, and backs it up
func (a *backupPVAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []actioninterface.ResourceIdentifier, error) {
	a.log.Info("Executing backupPVAction")

	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pvc); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert unstructured item to persistent volume claim")
	}

	if pvc.Status.Phase != corev1api.ClaimBound || pvc.Spec.VolumeName == "" {
		return item, nil, nil
	}

	pv := actioninterface.ResourceIdentifier{
		GroupResource: kuberesource.PersistentVolumes,
		Name:          pvc.Spec.VolumeName,
	}
	return item, []actioninterface.ResourceIdentifier{pv}, nil
}
