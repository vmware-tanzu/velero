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

	"github.com/google/uuid"
	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

// volumeSnapshotContentDeleteItemAction is a restore item action plugin for Velero
type volumeSnapshotContentDeleteItemAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

// AppliesTo returns information indicating
// VolumeSnapshotContentRestoreItemAction action should be invoked
// while restoring VolumeSnapshotContent.snapshot.storage.k8s.io resources
func (*volumeSnapshotContentDeleteItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"volumesnapshotcontents.snapshot.storage.k8s.io"},
	}, nil
}

func (p *volumeSnapshotContentDeleteItemAction) Execute(
	input *velero.DeleteItemActionExecuteInput,
) error {
	p.log.Info("Starting VolumeSnapshotContentDeleteItemAction")

	var snapCont snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(),
		&snapCont,
	); err != nil {
		return errors.Wrapf(err, "failed to convert VolumeSnapshotContent from unstructured")
	}

	// We don't want this DeleteItemAction plugin to delete
	// VolumeSnapshotContent taken outside of Velero.
	// So skip deleting VolumeSnapshotContent not have the backup name
	// in its labels.
	if !kubeutil.HasBackupLabel(&snapCont.ObjectMeta, input.Backup.Name) {
		p.log.Info(
			"VolumeSnapshotContent %s was not taken by backup %s, skipping deletion",
			snapCont.Name,
			input.Backup.Name,
		)
		return nil
	}

	p.log.Infof("Deleting VolumeSnapshotContent %s", snapCont.Name)

	uuid, err := uuid.NewRandom()
	if err != nil {
		p.log.WithError(err).Errorf("Fail to generate the UUID to create VSC %s", snapCont.Name)
		return errors.Wrapf(err, "Fail to generate the UUID to create VSC %s", snapCont.Name)
	}
	snapCont.Name = "vsc-" + uuid.String()

	snapCont.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentDelete

	snapCont.Spec.Source = snapshotv1api.VolumeSnapshotContentSource{
		SnapshotHandle: snapCont.Status.SnapshotHandle,
	}

	snapCont.Spec.VolumeSnapshotRef = corev1api.ObjectReference{
		APIVersion: snapshotv1api.SchemeGroupVersion.String(),
		Kind:       "VolumeSnapshot",
		Namespace:  "ns-" + string(snapCont.UID),
		Name:       "name-" + string(snapCont.UID),
	}

	snapCont.ResourceVersion = ""

	if err := p.crClient.Create(context.TODO(), &snapCont); err != nil {
		return errors.Wrapf(err, "fail to create VolumeSnapshotContent %s", snapCont.Name)
	}

	// Read resource timeout from backup annotation, if not set, use default value.
	timeout, err := time.ParseDuration(
		input.Backup.Annotations[velerov1api.ResourceTimeoutAnnotation])
	if err != nil {
		p.log.Warnf("fail to parse resource timeout annotation %s: %s",
			input.Backup.Annotations[velerov1api.ResourceTimeoutAnnotation], err.Error())
		timeout = 10 * time.Minute
	}
	p.log.Debugf("resource timeout is set to %s", timeout.String())

	interval := 5 * time.Second

	// Wait until VSC created and ReadyToUse is true.
	if err := wait.PollUntilContextTimeout(
		context.Background(),
		interval,
		timeout,
		true,
		func(ctx context.Context) (bool, error) {
			return checkVSCReadiness(ctx, &snapCont, p.crClient)
		},
	); err != nil {
		return errors.Wrapf(err, "fail to wait VolumeSnapshotContent %s becomes ready.", snapCont.Name)
	}

	if err := p.crClient.Delete(
		context.TODO(),
		&snapCont,
	); err != nil && !apierrors.IsNotFound(err) {
		p.log.Infof("VolumeSnapshotContent %s not found", snapCont.Name)
		return err
	}

	return nil
}

var checkVSCReadiness = func(
	ctx context.Context,
	vsc *snapshotv1api.VolumeSnapshotContent,
	client crclient.Client,
) (bool, error) {
	tmpVSC := new(snapshotv1api.VolumeSnapshotContent)
	if err := client.Get(ctx, crclient.ObjectKeyFromObject(vsc), tmpVSC); err != nil {
		return false, errors.Wrapf(
			err, "failed to get VolumeSnapshotContent %s", vsc.Name,
		)
	}

	if tmpVSC.Status != nil && boolptr.IsSetToTrue(tmpVSC.Status.ReadyToUse) {
		return true, nil
	}

	return false, nil
}

func NewVolumeSnapshotContentDeleteItemAction(
	f client.Factory,
) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, err
		}

		return &volumeSnapshotContentDeleteItemAction{
			log:      logger,
			crClient: crClient,
		}, nil
	}
}
