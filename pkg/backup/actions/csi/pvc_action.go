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
	"strconv"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/label"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/utils/volumehelper"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	biav2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/backupitemaction/v2"
	uploaderUtil "github.com/vmware-tanzu/velero/pkg/uploader/util"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

// pvcBackupItemAction is a backup item action plugin for Velero.
type pvcBackupItemAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

// AppliesTo returns information indicating that the PVCBackupItemAction
// should be invoked to backup PVCs.
func (p *pvcBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

func (p *pvcBackupItemAction) validateBackup(backup velerov1api.Backup) (valid bool) {
	if backup.Status.Phase == velerov1api.BackupPhaseFinalizing ||
		backup.Status.Phase == velerov1api.BackupPhaseFinalizingPartiallyFailed {
		p.log.WithFields(
			logrus.Fields{
				"Backup": fmt.Sprintf("%s/%s", backup.Namespace, backup.Name),
				"Phase":  backup.Status.Phase,
			},
		).Debug("Backup is in finalizing phase. Skip this PVC.")
		return false
	}

	return true
}

func (p *pvcBackupItemAction) validatePVCandPV(
	pvc corev1api.PersistentVolumeClaim,
	item runtime.Unstructured,
) (
	valid bool,
	updateItem runtime.Unstructured,
	err error,
) {
	updateItem = item

	// no storage class: we don't know how to map to a VolumeSnapshotClass
	if pvc.Spec.StorageClassName == nil {
		return false,
			updateItem,
			errors.Errorf(
				"Cannot snapshot PVC %s/%s, PVC has no storage class.",
				pvc.Namespace, pvc.Name)
	}

	p.log.Debugf(
		"Fetching underlying PV for PVC %s",
		fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
	)

	// Do nothing if this is not a CSI provisioned volume
	pv, err := kubeutil.GetPVForPVC(&pvc, p.crClient)
	if err != nil {
		return false, updateItem, errors.WithStack(err)
	}

	if pv.Spec.PersistentVolumeSource.CSI == nil {
		p.log.Infof(
			"Skipping PVC %s/%s, associated PV %s is not a CSI volume",
			pvc.Namespace, pvc.Name, pv.Name)

		kubeutil.AddAnnotations(
			&pvc.ObjectMeta,
			map[string]string{
				velerov1api.SkippedNoCSIPVAnnotation: "true",
			})
		data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pvc)
		updateItem = &unstructured.Unstructured{Object: data}
		return false, updateItem, err
	}

	return true, updateItem, nil
}

func (p *pvcBackupItemAction) createVolumeSnapshot(
	pvc corev1api.PersistentVolumeClaim,
	backup *velerov1api.Backup,
) (
	vs *snapshotv1api.VolumeSnapshot,
	err error,
) {
	p.log.Debugf("Fetching storage class for PV %s", *pvc.Spec.StorageClassName)
	storageClass := new(storagev1api.StorageClass)
	if err := p.crClient.Get(
		context.TODO(), crclient.ObjectKey{Name: *pvc.Spec.StorageClassName},
		storageClass,
	); err != nil {
		return nil, errors.Wrap(err, "error getting storage class")
	}

	p.log.Debugf("Fetching VolumeSnapshotClass for %s", storageClass.Provisioner)
	vsClass, err := csi.GetVolumeSnapshotClass(
		storageClass.Provisioner,
		backup,
		&pvc,
		p.log,
		p.crClient,
	)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to get VolumeSnapshotClass for StorageClass %s",
			storageClass.Name,
		)
	}
	p.log.Infof("VolumeSnapshotClass=%s", vsClass.Name)

	vsLabels := map[string]string{}
	for k, v := range pvc.ObjectMeta.Labels {
		vsLabels[k] = v
	}
	vsLabels[velerov1api.BackupNameLabel] = label.GetValidName(backup.Name)

	// Craft the vs object to be created
	vs = &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "velero-" + pvc.Name + "-",
			Namespace:    pvc.Namespace,
			Labels:       vsLabels,
		},
		Spec: snapshotv1api.VolumeSnapshotSpec{
			Source: snapshotv1api.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
			VolumeSnapshotClassName: &vsClass.Name,
		},
	}

	if err := p.crClient.Create(context.TODO(), vs); err != nil {
		return nil, errors.Wrapf(
			err, "error creating volume snapshot",
		)
	}
	p.log.Infof(
		"Created VolumeSnapshot %s",
		fmt.Sprintf("%s/%s", vs.Namespace, vs.Name),
	)

	return vs, nil
}

// Execute recognizes PVCs backed by volumes provisioned by CSI drivers
// with VolumeSnapshotting capability and creates snapshots of the
// underlying PVs by creating VolumeSnapshot CSI API objects that will
// trigger the CSI driver to perform the snapshot operation on the volume.
func (p *pvcBackupItemAction) Execute(
	item runtime.Unstructured,
	backup *velerov1api.Backup,
) (
	runtime.Unstructured,
	[]velero.ResourceIdentifier,
	string,
	[]velero.ResourceIdentifier,
	error,
) {
	p.log.Info("Starting PVCBackupItemAction")

	if valid := p.validateBackup(*backup); !valid {
		return item, nil, "", nil, nil
	}

	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		item.UnstructuredContent(),
		&pvc,
	); err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}

	if valid, item, err := p.validatePVCandPV(
		pvc,
		item,
	); !valid {
		if err != nil {
			return nil, nil, "", nil, err
		}
		return item, nil, "", nil, nil
	}

	shouldSnapshot, err := volumehelper.ShouldPerformSnapshotWithBackup(
		item,
		kuberesource.PersistentVolumeClaims,
		*backup,
		p.crClient,
		p.log,
	)
	if err != nil {
		return nil, nil, "", nil, err
	}
	if !shouldSnapshot {
		p.log.Debugf("CSI plugin skip snapshot for PVC %s according to the VolumeHelper setting.",
			pvc.Namespace+"/"+pvc.Name)
		return nil, nil, "", nil, err
	}

	vs, err := p.createVolumeSnapshot(pvc, backup)
	if err != nil {
		return nil, nil, "", nil, err
	}

	labels := map[string]string{
		velerov1api.VolumeSnapshotLabel: vs.Name,
		velerov1api.BackupNameLabel:     backup.Name,
	}

	annotations := map[string]string{
		velerov1api.VolumeSnapshotLabel:                 vs.Name,
		velerov1api.MustIncludeAdditionalItemAnnotation: "true",
	}

	var additionalItems []velero.ResourceIdentifier
	operationID := ""
	var itemToUpdate []velero.ResourceIdentifier

	if boolptr.IsSetToTrue(backup.Spec.SnapshotMoveData) {
		operationID = label.GetValidName(
			string(
				velerov1api.AsyncOperationIDPrefixDataUpload,
			) + string(backup.UID) + "." + string(pvc.UID),
		)
		dataUploadLog := p.log.WithFields(logrus.Fields{
			"Source PVC":     fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
			"VolumeSnapshot": fmt.Sprintf("%s/%s", vs.Namespace, vs.Name),
			"Operation ID":   operationID,
			"Backup":         backup.Name,
		})

		// Wait until VS associated VSC snapshot handle created before
		// returning with the Async operation for data mover.
		_, err := csi.WaitUntilVSCHandleIsReady(
			vs,
			p.crClient,
			p.log,
			true,
			backup.Spec.CSISnapshotTimeout.Duration,
		)
		if err != nil {
			dataUploadLog.Errorf(
				"Fail to wait VolumeSnapshot turned to ReadyToUse: %s",
				err.Error(),
			)
			csi.CleanupVolumeSnapshot(vs, p.crClient, p.log)
			return nil, nil, "", nil, errors.WithStack(err)
		}

		dataUploadLog.Info("Starting data upload of backup")

		dataUpload, err := createDataUpload(
			context.Background(),
			backup,
			p.crClient,
			vs,
			&pvc,
			operationID,
		)
		if err != nil {
			dataUploadLog.WithError(err).Error("failed to submit DataUpload")

			// TODO: need to use DeleteVolumeSnapshotIfAny, after data mover
			// adopting the controller-runtime client.
			if deleteErr := p.crClient.Delete(context.TODO(), vs); deleteErr != nil {
				if !apierrors.IsNotFound(deleteErr) {
					dataUploadLog.WithError(deleteErr).Error("fail to delete VolumeSnapshot")
				}
			}

			// Return without modification to not fail the backup,
			// and the above error log makes the backup partially fail.
			return item, nil, "", nil, nil
		} else {
			itemToUpdate = []velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{
						Group:    "velero.io",
						Resource: "datauploads",
					},
					Namespace: dataUpload.Namespace,
					Name:      dataUpload.Name,
				},
			}
			// Set the DataUploadNameLabel, which is used for restore to
			// let CSI plugin check whether it should handle the volume.
			// If volume is CSI migration, PVC doesn't have the annotation.
			annotations[velerov1api.DataUploadNameAnnotation] = dataUpload.Namespace + "/" + dataUpload.Name

			dataUploadLog.Info("DataUpload is submitted successfully.")
		}
	} else {
		additionalItems = []velero.ResourceIdentifier{
			{
				GroupResource: kuberesource.VolumeSnapshots,
				Namespace:     vs.Namespace,
				Name:          vs.Name,
			},
		}
	}

	kubeutil.AddAnnotations(&pvc.ObjectMeta, annotations)
	kubeutil.AddLabels(&pvc.ObjectMeta, labels)

	p.log.Infof("Returning from PVCBackupItemAction with %d additionalItems to backup",
		len(additionalItems))
	for _, ai := range additionalItems {
		p.log.Debugf("%s: %s", ai.GroupResource.String(), ai.Name)
	}

	pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pvc)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: pvcMap},
		additionalItems, operationID, itemToUpdate, nil
}

func (p *pvcBackupItemAction) Name() string {
	return "PVCBackupItemAction"
}

func (p *pvcBackupItemAction) Progress(
	operationID string,
	backup *velerov1api.Backup,
) (velero.OperationProgress, error) {
	progress := velero.OperationProgress{}
	if operationID == "" {
		return progress, biav2.InvalidOperationIDError(operationID)
	}

	dataUpload, err := getDataUpload(context.Background(), p.crClient, operationID)
	if err != nil {
		p.log.Errorf(
			"fail to get DataUpload for backup %s/%s by operation ID %s: %s",
			backup.Namespace, backup.Name, operationID, err.Error(),
		)
		return progress, err
	}
	if dataUpload.Status.Phase == velerov2alpha1.DataUploadPhaseNew ||
		dataUpload.Status.Phase == "" {
		p.log.Debugf("DataUpload is still not processed yet. Skip progress update.")
		return progress, nil
	}

	progress.Description = string(dataUpload.Status.Phase)
	progress.OperationUnits = "Bytes"
	progress.NCompleted = dataUpload.Status.Progress.BytesDone
	progress.NTotal = dataUpload.Status.Progress.TotalBytes

	if dataUpload.Status.StartTimestamp != nil {
		progress.Started = dataUpload.Status.StartTimestamp.Time
	}

	if dataUpload.Status.CompletionTimestamp != nil {
		progress.Updated = dataUpload.Status.CompletionTimestamp.Time
	}

	if dataUpload.Status.Phase == velerov2alpha1.DataUploadPhaseCompleted {
		progress.Completed = true
	} else if dataUpload.Status.Phase == velerov2alpha1.DataUploadPhaseFailed {
		progress.Completed = true
		progress.Err = dataUpload.Status.Message
	} else if dataUpload.Status.Phase == velerov2alpha1.DataUploadPhaseCanceled {
		progress.Completed = true
		progress.Err = "DataUpload is canceled"
	}

	return progress, nil
}

func (p *pvcBackupItemAction) Cancel(operationID string, backup *velerov1api.Backup) error {
	if operationID == "" {
		return biav2.InvalidOperationIDError(operationID)
	}

	dataUpload, err := getDataUpload(context.Background(), p.crClient, operationID)
	if err != nil {
		p.log.Errorf(
			"fail to get DataUpload for backup %s/%s: %s",
			backup.Namespace, backup.Name, err.Error(),
		)
		return err
	}

	return cancelDataUpload(context.Background(), p.crClient, dataUpload)
}

func newDataUpload(
	backup *velerov1api.Backup,
	vs *snapshotv1api.VolumeSnapshot,
	pvc *corev1api.PersistentVolumeClaim,
	operationID string,
) *velerov2alpha1.DataUpload {
	dataUpload := &velerov2alpha1.DataUpload{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov2alpha1.SchemeGroupVersion.String(),
			Kind:       "DataUpload",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    backup.Namespace,
			GenerateName: backup.Name + "-",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: velerov1api.SchemeGroupVersion.String(),
					Kind:       "Backup",
					Name:       backup.Name,
					UID:        backup.UID,
					Controller: boolptr.True(),
				},
			},
			Labels: map[string]string{
				velerov1api.BackupNameLabel:       label.GetValidName(backup.Name),
				velerov1api.BackupUIDLabel:        string(backup.UID),
				velerov1api.PVCUIDLabel:           string(pvc.UID),
				velerov1api.AsyncOperationIDLabel: operationID,
			},
		},
		Spec: velerov2alpha1.DataUploadSpec{
			SnapshotType: velerov2alpha1.SnapshotTypeCSI,
			CSISnapshot: &velerov2alpha1.CSISnapshotSpec{
				VolumeSnapshot: vs.Name,
				StorageClass:   *pvc.Spec.StorageClassName,
				SnapshotClass:  *vs.Spec.VolumeSnapshotClassName,
			},
			SourcePVC:             pvc.Name,
			DataMover:             backup.Spec.DataMover,
			BackupStorageLocation: backup.Spec.StorageLocation,
			SourceNamespace:       pvc.Namespace,
			OperationTimeout:      backup.Spec.CSISnapshotTimeout,
		},
	}

	if backup.Spec.UploaderConfig != nil &&
		backup.Spec.UploaderConfig.ParallelFilesUpload > 0 {
		dataUpload.Spec.DataMoverConfig = make(map[string]string)
		dataUpload.Spec.DataMoverConfig[uploaderUtil.ParallelFilesUpload] = strconv.Itoa(backup.Spec.UploaderConfig.ParallelFilesUpload)
	}

	return dataUpload
}

func createDataUpload(
	ctx context.Context,
	backup *velerov1api.Backup,
	crClient crclient.Client,
	vs *snapshotv1api.VolumeSnapshot,
	pvc *corev1api.PersistentVolumeClaim,
	operationID string,
) (*velerov2alpha1.DataUpload, error) {
	dataUpload := newDataUpload(backup, vs, pvc, operationID)

	err := crClient.Create(ctx, dataUpload)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create DataUpload CR")
	}

	return dataUpload, err
}

func getDataUpload(
	ctx context.Context,
	crClient crclient.Client,
	operationID string,
) (*velerov2alpha1.DataUpload, error) {
	dataUploadList := new(velerov2alpha1.DataUploadList)
	err := crClient.List(ctx, dataUploadList, &crclient.ListOptions{
		LabelSelector: labels.SelectorFromSet(
			map[string]string{velerov1api.AsyncOperationIDLabel: operationID},
		),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error to list DataUpload")
	}

	if len(dataUploadList.Items) == 0 {
		return nil, errors.Errorf("not found DataUpload for operationID %s", operationID)
	}

	if len(dataUploadList.Items) > 1 {
		return nil, errors.Errorf("more than one DataUpload found operationID %s", operationID)
	}

	return &dataUploadList.Items[0], nil
}

func cancelDataUpload(
	ctx context.Context,
	crClient crclient.Client,
	dataUpload *velerov2alpha1.DataUpload,
) error {
	updatedDataUpload := dataUpload.DeepCopy()
	updatedDataUpload.Spec.Cancel = true

	err := crClient.Patch(ctx, updatedDataUpload, crclient.MergeFrom(dataUpload))
	if err != nil {
		return errors.Wrap(err, "error patch DataUpload")
	}

	return nil
}

func NewPvcBackupItemAction(f client.Factory) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return &pvcBackupItemAction{
			log:      logger,
			crClient: crClient,
		}, nil
	}
}
