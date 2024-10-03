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
	"context"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/label"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
)

// volumeSnapshotRestoreItemAction is a Velero restore item
// action plugin for VolumeSnapshots
type volumeSnapshotRestoreItemAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

// AppliesTo returns information indicating that
// VolumeSnapshotRestoreItemAction should be invoked while
// restoring volumesnapshots.snapshot.storage.k8s.io resources.
func (p *volumeSnapshotRestoreItemAction) AppliesTo() (
	velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"volumesnapshots.snapshot.storage.k8s.io"},
	}, nil
}

func resetVolumeSnapshotSpecForRestore(
	vs *snapshotv1api.VolumeSnapshot, vscName *string) {
	// Spec of the backed-up object used the PVC as the source
	// of the volumeSnapshot.  Restore operation will however,
	// restore the VolumeSnapshot from the VolumeSnapshotContent
	vs.Spec.Source.PersistentVolumeClaimName = nil
	vs.Spec.Source.VolumeSnapshotContentName = vscName
}

func resetVolumeSnapshotAnnotation(vs *snapshotv1api.VolumeSnapshot) {
	vs.ObjectMeta.Annotations[velerov1api.VSCDeletionPolicyAnnotation] =
		string(snapshotv1api.VolumeSnapshotContentRetain)
}

// Execute uses the data such as CSI driver name, storage
// snapshot handle, snapshot deletion secret (if any) from
// the annotations to recreate a VolumeSnapshotContent object
// and statically bind the VolumeSnapshot object being restored.
func (p *volumeSnapshotRestoreItemAction) Execute(
	input *velero.RestoreItemActionExecuteInput,
) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Starting VolumeSnapshotRestoreItemAction")

	if boolptr.IsSetToFalse(input.Restore.Spec.RestorePVs) {
		p.log.Infof("Restore %s/%s did not request for PVs to be restored.",
			input.Restore.Namespace, input.Restore.Name)
		return &velero.RestoreItemActionExecuteOutput{SkipRestore: true}, nil
	}

	var vs snapshotv1api.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &vs); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	// If cross-namespace restore is configured, change the namespace
	// for VolumeSnapshot object to be restored
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[vs.GetNamespace()]
	if !ok {
		// Use original namespace
		newNamespace = vs.Namespace
	}

	if !csi.IsVolumeSnapshotExists(newNamespace, vs.Name, p.crClient) {
		snapHandle, exists := vs.Annotations[velerov1api.VolumeSnapshotHandleAnnotation]
		if !exists {
			return nil, errors.Errorf(
				"VolumeSnapshot %s/%s does not have a %s annotation",
				vs.Namespace,
				vs.Name,
				velerov1api.VolumeSnapshotHandleAnnotation,
			)
		}

		csiDriverName, exists := vs.Annotations[velerov1api.DriverNameAnnotation]
		if !exists {
			return nil, errors.Errorf(
				"VolumeSnapshot %s/%s does not have a %s annotation",
				vs.Namespace, vs.Name, velerov1api.DriverNameAnnotation)
		}

		p.log.Debugf("Set VolumeSnapshotContent %s/%s DeletionPolicy to Retain to make sure VS deletion in namespace will not delete Snapshot on cloud provider.",
			newNamespace, vs.Name)

		vsc := snapshotv1api.VolumeSnapshotContent{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: vs.Name + "-",
				Labels: map[string]string{
					velerov1api.RestoreNameLabel: label.GetValidName(input.Restore.Name),
				},
			},
			Spec: snapshotv1api.VolumeSnapshotContentSpec{
				DeletionPolicy: snapshotv1api.VolumeSnapshotContentRetain,
				Driver:         csiDriverName,
				VolumeSnapshotRef: core_v1.ObjectReference{
					APIVersion: "snapshot.storage.k8s.io/v1",
					Kind:       "VolumeSnapshot",
					Namespace:  newNamespace,
					Name:       vs.Name,
				},
				Source: snapshotv1api.VolumeSnapshotContentSource{
					SnapshotHandle: &snapHandle,
				},
			},
		}

		// we create the VolumeSnapshotContent here instead of relying on the
		// restore flow because we want to statically bind this VolumeSnapshot
		// with a VolumeSnapshotContent that will be used as its source for pre-populating
		// the volume that will be created as a result of the restore. To perform
		// this static binding, a bi-directional link between the VolumeSnapshotContent
		// and VolumeSnapshot objects have to be setup. Further, it is disallowed
		// to convert a dynamically created VolumeSnapshotContent for static binding.
		// See: https://github.com/kubernetes-csi/external-snapshotter/issues/274
		if err := p.crClient.Create(context.TODO(), &vsc); err != nil {
			return nil, errors.Wrapf(err,
				"failed to create VolumeSnapshotContents %s",
				vsc.GenerateName)
		}
		p.log.Infof("Created VolumesnapshotContents %s with static binding to volumesnapshot %s/%s",
			vsc, newNamespace, vs.Name)

		// Reset Spec to convert the VolumeSnapshot from using
		// the dynamic VolumeSnapshotContent to the static one.
		resetVolumeSnapshotSpecForRestore(&vs, &vsc.Name)

		// Reset VolumeSnapshot annotation. By now, only change
		// DeletionPolicy to Retain.
		resetVolumeSnapshotAnnotation(&vs)
	}

	vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&vs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.log.Infof(`Returning from VolumeSnapshotRestoreItemAction with 
		no additionalItems`)

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vsMap},
		AdditionalItems: []velero.ResourceIdentifier{},
	}, nil
}

func (p *volumeSnapshotRestoreItemAction) Name() string {
	return "VolumeSnapshotRestoreItemAction"
}

func (p *volumeSnapshotRestoreItemAction) Progress(
	operationID string,
	restore *velerov1api.Restore,
) (velero.OperationProgress, error) {
	return velero.OperationProgress{}, nil
}

func (p *volumeSnapshotRestoreItemAction) Cancel(
	operationID string,
	restore *velerov1api.Restore,
) error {
	return nil
}

func (p *volumeSnapshotRestoreItemAction) AreAdditionalItemsReady(
	additionalItems []velero.ResourceIdentifier,
	restore *velerov1api.Restore,
) (bool, error) {
	return true, nil
}

func NewVolumeSnapshotRestoreItemAction(
	f client.Factory) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (interface{}, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, err
		}

		return &volumeSnapshotRestoreItemAction{logger, crClient}, nil
	}
}
