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
	"fmt"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/client"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

// volumeSnapshotDeleteItemAction is a backup item action plugin for Velero.
type volumeSnapshotDeleteItemAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

// AppliesTo returns information indicating that the
// VolumeSnapshotBackupItemAction should be invoked to backup
// VolumeSnapshots.
func (p *volumeSnapshotDeleteItemAction) AppliesTo() (velero.ResourceSelector, error) {
	p.log.Debug("VolumeSnapshotBackupItemAction AppliesTo")

	return velero.ResourceSelector{
		IncludedResources: []string{"volumesnapshots.snapshot.storage.k8s.io"},
	}, nil
}

func (p *volumeSnapshotDeleteItemAction) Execute(
	input *velero.DeleteItemActionExecuteInput,
) error {
	p.log.Info("Starting VolumeSnapshotDeleteItemAction for volumeSnapshot")

	var vs snapshotv1api.VolumeSnapshot

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(),
		&vs,
	); err != nil {
		return errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	// We don't want this DeleteItemAction plugin to delete VolumeSnapshot
	// taken outside of Velero.  So skip deleting VolumeSnapshot objects
	// that were not created in the process of creating the Velero
	// backup being deleted.
	if !kubeutil.HasBackupLabel(&vs.ObjectMeta, input.Backup.Name) {
		p.log.Info(
			"VolumeSnapshot %s/%s was not taken by backup %s, skipping deletion",
			vs.Namespace, vs.Name, input.Backup.Name,
		)
		return nil
	}

	p.log.Infof("Deleting VolumeSnapshot %s/%s", vs.Namespace, vs.Name)
	if vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil {
		// we patch the DeletionPolicy of the VolumeSnapshotContent
		// to set it to Delete.  This ensures that the volume snapshot
		// in the storage provider is also deleted.
		err := csi.SetVolumeSnapshotContentDeletionPolicy(
			*vs.Status.BoundVolumeSnapshotContentName,
			p.crClient,
		)
		if err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(
				err,
				fmt.Sprintf("failed to patch DeletionPolicy of volume snapshot %s/%s",
					vs.Namespace, vs.Name),
			)
		}

		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	err := p.crClient.Delete(context.TODO(), &vs)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func NewVolumeSnapshotDeleteItemAction(f client.Factory) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return &volumeSnapshotDeleteItemAction{
			log:      logger,
			crClient: crClient,
		}, nil
	}
}
