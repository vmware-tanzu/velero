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
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
)

// volumeSnapshotContentRestoreItemAction is a restore item action
// plugin for Velero
type volumeSnapshotContentRestoreItemAction struct {
	log logrus.FieldLogger
}

// AppliesTo returns information indicating VolumeSnapshotContentRestoreItemAction
// action should be invoked while restoring
// volumesnapshotcontent.snapshot.storage.k8s.io resources
func (p *volumeSnapshotContentRestoreItemAction) AppliesTo() (
	velero.ResourceSelector, error,
) {
	return velero.ResourceSelector{
		IncludedResources: []string{"volumesnapshotcontents.snapshot.storage.k8s.io"},
	}, nil
}

// Execute restores a VolumeSnapshotContent object without modification
// returning the snapshot lister secret, if any, as additional items to restore.
func (p *volumeSnapshotContentRestoreItemAction) Execute(
	input *velero.RestoreItemActionExecuteInput,
) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Starting VolumeSnapshotContentRestoreItemAction")
	var snapCont snapshotv1api.VolumeSnapshotContent
	if boolptr.IsSetToFalse(input.Restore.Spec.RestorePVs) {
		p.log.Infof("Restore did not request for PVs to be restored %s/%s",
			input.Restore.Namespace, input.Restore.Name)
		return &velero.RestoreItemActionExecuteOutput{SkipRestore: true}, nil
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &snapCont); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	additionalItems := []velero.ResourceIdentifier{}
	if csi.IsVolumeSnapshotContentHasDeleteSecret(&snapCont) {
		additionalItems = append(additionalItems,
			velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
				Name:          snapCont.Annotations[velerov1api.PrefixedSecretNameAnnotation],
				Namespace:     snapCont.Annotations[velerov1api.PrefixedSecretNamespaceAnnotation],
			},
		)
	}

	p.log.Infof("Returning from VolumeSnapshotContentRestoreItemAction with %d additionalItems",
		len(additionalItems))
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     input.Item,
		AdditionalItems: additionalItems,
	}, nil
}

func (p *volumeSnapshotContentRestoreItemAction) Name() string {
	return "VolumeSnapshotContentRestoreItemAction"
}

func (p *volumeSnapshotContentRestoreItemAction) Progress(
	operationID string,
	restore *velerov1api.Restore,
) (velero.OperationProgress, error) {
	return velero.OperationProgress{}, nil
}

func (p *volumeSnapshotContentRestoreItemAction) Cancel(
	operationID string,
	restore *velerov1api.Restore,
) error {
	return nil
}

func (p *volumeSnapshotContentRestoreItemAction) AreAdditionalItemsReady(
	additionalItems []velero.ResourceIdentifier,
	restore *velerov1api.Restore,
) (bool, error) {
	return true, nil
}

func NewVolumeSnapshotContentRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	return &volumeSnapshotContentRestoreItemAction{logger}, nil
}
