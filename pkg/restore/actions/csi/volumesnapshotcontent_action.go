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
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
)

// volumeSnapshotContentRestoreItemAction is a restore item action
// plugin for Velero
type volumeSnapshotContentRestoreItemAction struct {
	log    logrus.FieldLogger
	client crclient.Client
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
	if boolptr.IsSetToFalse(input.Restore.Spec.RestorePVs) {
		p.log.Infof("Restore did not request for PVs to be restored %s/%s",
			input.Restore.Namespace, input.Restore.Name)
		return &velero.RestoreItemActionExecuteOutput{SkipRestore: true}, nil
	}

	p.log.Info("Starting VolumeSnapshotContentRestoreItemAction")

	var snapCont snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &snapCont); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	if err := p.ensureDeleteExistingVSC(snapCont); err != nil {
		p.log.Errorf("fail to delete the existing VSC: %s", err.Error())
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	var vscFromBackup snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &vscFromBackup); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Errorf(err.Error(), "failed to convert input.ItemFromBackup from unstructured")
	}

	// If cross-namespace restore is configured, change the namespace
	// for VolumeSnapshot object to be restored
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[snapCont.Spec.VolumeSnapshotRef.Namespace]
	if ok {
		// Update the referenced VS namespace to the mapping one.
		snapCont.Spec.VolumeSnapshotRef.Namespace = newNamespace
	}

	// Reset the ResourceVersion and UID of referenced VolumeSnapshot.
	snapCont.Spec.VolumeSnapshotRef.ResourceVersion = ""
	snapCont.Spec.VolumeSnapshotRef.UID = ""

	// Set the DeletionPolicy to Retain to avoid VS deletion will not trigger snapshot deletion
	snapCont.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain

	if vscFromBackup.Status != nil && vscFromBackup.Status.SnapshotHandle != nil {
		snapCont.Spec.Source.VolumeHandle = nil
		snapCont.Spec.Source.SnapshotHandle = vscFromBackup.Status.SnapshotHandle
	} else {
		p.log.Errorf("fail to get snapshot handle from VSC %s status", snapCont.Name)
		return nil, errors.Errorf("fail to get snapshot handle from VSC %s status", snapCont.Name)
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

	vscMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&snapCont)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.log.Infof("Returning from VolumeSnapshotContentRestoreItemAction with %d additionalItems",
		len(additionalItems))
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vscMap},
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

func NewVolumeSnapshotContentRestoreItemAction(
	f client.Factory,
) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, err
		}

		return &volumeSnapshotContentRestoreItemAction{logger, crClient}, nil
	}
}

func (p *volumeSnapshotContentRestoreItemAction) ensureDeleteExistingVSC(
	vsc snapshotv1api.VolumeSnapshotContent,
) error {
	err := p.client.Delete(context.TODO(), &vsc)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "error to delete volume snapshot content")
	}

	var updated snapshotv1api.VolumeSnapshotContent
	err = wait.PollUntilContextTimeout(
		context.TODO(),
		2*time.Second,
		5*time.Minute,
		true,
		func(ctx context.Context) (bool, error) {
			if err := p.client.Get(ctx, crclient.ObjectKeyFromObject(&vsc), &updated); err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}

				return false, errors.Errorf("error to get VolumeSnapshotContent %s: %s", vsc.Name, err.Error())
			}

			return false, nil
		},
	)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.Errorf("timeout to assure VolumeSnapshotContent %s is deleted, finalizers in VSC %v", vsc.Name, updated.Finalizers)
		} else {
			return errors.Wrapf(err, "error to assure VolumeSnapshotContent is deleted, %s", vsc.Name)
		}
	}

	return nil
}
