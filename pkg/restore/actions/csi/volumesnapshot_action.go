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
	"strings"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	riav2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/restoreitemaction/v2"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
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

	var vsFromBackup snapshotv1api.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &vsFromBackup); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Errorf(err.Error(), "failed to convert input.ItemFromBackup from unstructured")
	}

	// If cross-namespace restore is configured, change the namespace
	// for VolumeSnapshot object to be restored
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[vs.GetNamespace()]
	if !ok {
		// Use original namespace
		newNamespace = vs.Namespace
	}

	// operationID format is "vsNamespace/vsNamespace".
	operationID := newNamespace + "/" + vs.Name

	// Reset Spec to convert the VolumeSnapshot from using
	// the dynamic VolumeSnapshotContent to the static one.
	if vsFromBackup.Status != nil && vsFromBackup.Status.BoundVolumeSnapshotContentName != nil {
		resetVolumeSnapshotSpecForRestore(&vs, vsFromBackup.Status.BoundVolumeSnapshotContentName)
	} else {
		p.log.Errorf("Fail to get VSC name from the VS %s status", operationID)
		return nil, errors.Errorf("fail to get VSC name from the VS %s status", operationID)
	}

	// Reset VolumeSnapshot annotation. By now, only change
	// DeletionPolicy to Retain.
	resetVolumeSnapshotAnnotation(&vs)

	vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&vs)
	if err != nil {
		p.log.Errorf("Fail to convert VS %s to unstructured", operationID)
		return nil, errors.WithStack(err)
	}

	p.log.Infof(`Returning from VolumeSnapshotRestoreItemAction with 
		no additionalItems`)

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vsMap},
		AdditionalItems: []velero.ResourceIdentifier{},
		OperationID:     operationID,
	}, nil
}

func (p *volumeSnapshotRestoreItemAction) Name() string {
	return "VolumeSnapshotRestoreItemAction"
}

func (p *volumeSnapshotRestoreItemAction) Progress(
	operationID string,
	restore *velerov1api.Restore,
) (velero.OperationProgress, error) {
	progress := velero.OperationProgress{}

	// The operationID format should be "vsNamespace/vsName".
	if operationID == "" || !strings.Contains(operationID, "/") {
		return progress, riav2.InvalidOperationIDError(operationID)
	}

	logger := p.log.WithFields(logrus.Fields{
		"Action":      "VolumeSnapshotRestoreItemAction",
		"OperationID": operationID,
		"Namespace":   restore.Namespace,
	})

	vsc := new(snapshotv1api.VolumeSnapshotContent)
	vs := new(snapshotv1api.VolumeSnapshot)

	operationIDParts := strings.Split(operationID, "/")

	if err := p.crClient.Get(
		context.TODO(),
		crclient.ObjectKey{Namespace: operationIDParts[0], Name: operationIDParts[1]},
		vs,
	); err != nil {
		logger.Errorf("Fail to get the VolumeSnapshot %s", operationID)
		return progress, err
	}

	if vs.Spec.Source.VolumeSnapshotContentName == nil {
		logger.Errorf("VolumeSnapshot %s referenced VSC is nil", operationID)
		return progress, errors.Errorf("VolumeSnapshot %s referenced VSC is nil", operationID)
	}

	if err := p.crClient.Get(
		context.TODO(),
		crclient.ObjectKey{
			Name: *vs.Spec.Source.VolumeSnapshotContentName,
		},
		vsc,
	); err != nil {
		logger.Errorf("Fail to get the VolumeSnapshotContent %s",
			*vs.Spec.Source.VolumeSnapshotContentName,
		)
		return progress, err
	}

	if (vsc.Status != nil && boolptr.IsSetToTrue(vsc.Status.ReadyToUse)) &&
		(vs.Status != nil && boolptr.IsSetToTrue(vs.Status.ReadyToUse)) {
		// VS and VSC are ready. Delete them to make sure they will not cause conflict for the next restore.
		if err := p.crClient.Delete(context.TODO(), vs); err != nil {
			logger.Warnf("Fail to delete VS %s after VS turns ready %s.", vs.Namespace+vs.Name, err.Error())
		}
		if err := p.crClient.Delete(context.TODO(), vsc); err != nil {
			logger.Warnf("Fail to delete VSC %s after VSC turns ready %s.", vsc.Name, err.Error())
		}
	} else {
		logger.Debugf("VolumeSnapshot or VolumeSnapshotContent are not ready yet.")
		progress.Description = "VolumeSnapshot or VolumeSnapshotContent creation are still in progress."
		return progress, nil
	}

	progress.Description = "VolumeSnapshot and VolumeSnapshotContent are ready"
	progress.OperationUnits = "Bytes"
	progress.Completed = true
	if vsc.Status.RestoreSize != nil {
		progress.NCompleted = *vsc.Status.RestoreSize
		progress.NTotal = *vsc.Status.RestoreSize
	}

	logger.Infof("VolumeSnapshot %s/%s and VolumeSnapshotContent %s are ready.",
		vs.Namespace, vs.Name, vsc.Name)

	return progress, nil
}

func (p *volumeSnapshotRestoreItemAction) Cancel(
	operationID string,
	restore *velerov1api.Restore,
) error {
	// CSI Specification doesn't support canceling a snapshot creation.
	return nil
}

func (p *volumeSnapshotRestoreItemAction) AreAdditionalItemsReady(
	additionalItems []velero.ResourceIdentifier,
	restore *velerov1api.Restore,
) (bool, error) {
	return true, nil
}

func NewVolumeSnapshotRestoreItemAction(
	f client.Factory,
) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, err
		}

		return &volumeSnapshotRestoreItemAction{logger, crClient}, nil
	}
}
