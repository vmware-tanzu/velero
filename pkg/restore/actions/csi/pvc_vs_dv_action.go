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

package csi

import (
    "github.com/pkg/errors"
    "github.com/sirupsen/logrus"
    corev1api "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/runtime/schema"

    velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
    "github.com/vmware-tanzu/velero/pkg/client"
    plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
    "github.com/vmware-tanzu/velero/pkg/plugin/velero"
    kuberesource "github.com/vmware-tanzu/velero/pkg/kuberesource"
)

// pvcVSDVRestoreItemAction is a restore item action plugin for Velero
type pvcVSDVRestoreItemAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

// AppliesTo returns information indicating that the
func (p *pvcVSDVRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},

	}, nil
}


// Execute modifies the PVC's spec to use the VolumeSnapshot object as the
// data source ensuring that the newly provisioned volume can be pre-populated
// with data from the VolumeSnapshot.
func (p *pvcVSDVRestoreItemAction) Execute(
	input *velero.RestoreItemActionExecuteInput,
) (*velero.RestoreItemActionExecuteOutput, error) {
	var pvcFromBackup corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &pvcFromBackup); err != nil {
		return nil, errors.WithStack(err)
	}

	logger := p.log.WithFields(logrus.Fields{
		"Action":  "PVCVSDVRestoreItemAction",
		"PVC":     pvcFromBackup.Namespace + "/" + pvcFromBackup.Name,
		"Restore": input.Restore.Namespace + "/" + input.Restore.Name,
	})
	logger.Info("Starting PVCVSDVRestoreItemAction for PVC")

	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.Wrap(err, "failed to convert PVC from unstructured")
	}

	additionalItems := []velero.ResourceIdentifier{}

	// Check for VolumeSnapshot annotation.
	// Velero uses velerov1api.VolumeSnapshotLabel to store the snapshot name.
	if pvc.Annotations != nil {
		if vsName, ok := pvc.Annotations[velerov1api.VolumeSnapshotLabel]; ok && vsName != "" {
			additionalItems = append(additionalItems, velero.ResourceIdentifier{
				GroupResource: kuberesource.VolumeSnapshots,
				Name:          vsName,
				Namespace:     pvc.Namespace,
			})
		}

		// Check for DataVolume annotation.
		if dvName, ok := pvc.Annotations[velerov1api.DataUploadNameAnnotation]; ok && dvName != "" {
			additionalItems = append(additionalItems, velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{
					Group:    "cdi.kubevirt.io",
					Resource: "datavolumes",
				},
				Name:      dvName,
				Namespace: pvc.Namespace,
			})
		}
	}

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     input.Item,
		AdditionalItems: additionalItems,
	}, nil
}



func (p *pvcVSDVRestoreItemAction) Name() string {
	return "PVCRestoreItemAction"
}


func (p *pvcVSDVRestoreItemAction) Cancel(
	operationID string, restore *velerov1api.Restore) error {
	return nil
}

func (p *pvcVSDVRestoreItemAction) AreAdditionalItemsReady(
	additionalItems []velero.ResourceIdentifier,
	restore *velerov1api.Restore,
) (bool, error) {
	return true, nil
}

// NewPvcVSDVRestoreItemAction returns a new instance of PVCVSDVRestoreItemAction.
func NewPvcVSDVRestoreItemAction(f client.Factory) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, err
		}

		return &pvcVSDVRestoreItemAction{
			log:      logger,
			crClient: crClient,
		}, nil
	}
}
