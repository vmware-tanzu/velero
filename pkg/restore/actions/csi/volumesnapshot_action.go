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
	"fmt"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util"
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
func (*volumeSnapshotRestoreItemAction) AppliesTo() (
	velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"volumesnapshots.snapshot.storage.k8s.io"},
	}, nil
}

func resetVolumeSnapshotSpecForRestore(vs *snapshotv1api.VolumeSnapshot, vscName *string) {
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
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	generatedName := util.GenerateSha256FromRestoreUIDAndVsName(string(input.Restore.UID), vsFromBackup.Name)

	// Reset Spec to convert the VolumeSnapshot from using
	// the dynamic VolumeSnapshotContent to the static one.
	resetVolumeSnapshotSpecForRestore(&vs, &generatedName)
	// Also reset the VS name to avoid potential conflict caused by multiple restores of the same backup.
	// Both VS and VSC share the same generated name.
	vs.Name = generatedName

	// Reset VolumeSnapshot annotation. By now, only change
	// DeletionPolicy to Retain.
	resetVolumeSnapshotAnnotation(&vs)

	vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&vs)
	if err != nil {
		p.log.Errorf("Fail to convert VS %s to unstructured", vs.Namespace+"/"+vs.Name)
		return nil, errors.WithStack(err)
	}

	if vsFromBackup.Status == nil ||
		vsFromBackup.Status.BoundVolumeSnapshotContentName == nil {
		p.log.Errorf("VS %s doesn't have bound VSC", vsFromBackup.Name)
		return nil, fmt.Errorf("VS %s doesn't have bound VSC", vsFromBackup.Name)
	}

	vsc := velero.ResourceIdentifier{
		GroupResource: kuberesource.VolumeSnapshotContents,
		Name:          *vsFromBackup.Status.BoundVolumeSnapshotContentName,
	}

	p.log.Infof(`Returning from VolumeSnapshotRestoreItemAction with 
		VolumeSnapshotContent in additionalItems`)

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vsMap},
		AdditionalItems: []velero.ResourceIdentifier{vsc},
	}, nil
}

func (*volumeSnapshotRestoreItemAction) Name() string {
	return "VolumeSnapshotRestoreItemAction"
}

func (*volumeSnapshotRestoreItemAction) Progress(
	string,
	*velerov1api.Restore,
) (velero.OperationProgress, error) {
	return velero.OperationProgress{}, nil
}

func (*volumeSnapshotRestoreItemAction) Cancel(
	string,
	*velerov1api.Restore,
) error {
	// CSI Specification doesn't support canceling a snapshot creation.
	return nil
}

func (*volumeSnapshotRestoreItemAction) AreAdditionalItemsReady(
	[]velero.ResourceIdentifier,
	*velerov1api.Restore,
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
