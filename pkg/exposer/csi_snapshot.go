/*
Copyright The Velero Contributors.

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

package exposer

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"

	"github.com/vmware-tanzu/velero/pkg/util/csi"
	"github.com/vmware-tanzu/velero/pkg/util/kube"

	snapshotter "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/typed/volumesnapshot/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CSISnapshotExposeParam define the input param for Expose of CSI snapshots
type CSISnapshotExposeParam struct {
	// SnapshotName is the original volume snapshot name
	SnapshotName string

	// SourceNamespace is the original namespace of the volume that the snapshot is taken for
	SourceNamespace string

	// AccessMode defines the mode to access the snapshot
	AccessMode string

	// StorageClass is the storage class of the volume that the snapshot is taken for
	StorageClass string

	// HostingPodLabels is the labels that are going to apply to the hosting pod
	HostingPodLabels map[string]string

	// Timeout specifies the time wait for resources operations in Expose
	Timeout time.Duration
}

// CSISnapshotExposeWaitParam define the input param for WaitExposed of CSI snapshots
type CSISnapshotExposeWaitParam struct {
	// NodeClient is the client that is used to find the hosting pod
	NodeClient client.Client
	NodeName   string
}

// NewCSISnapshotExposer create a new instance of CSI snapshot exposer
func NewCSISnapshotExposer(kubeClient kubernetes.Interface, csiSnapshotClient snapshotter.SnapshotV1Interface, log logrus.FieldLogger) SnapshotExposer {
	return &csiSnapshotExposer{
		kubeClient:        kubeClient,
		csiSnapshotClient: csiSnapshotClient,
		log:               log,
	}
}

type csiSnapshotExposer struct {
	kubeClient        kubernetes.Interface
	csiSnapshotClient snapshotter.SnapshotV1Interface
	log               logrus.FieldLogger
}

func (e *csiSnapshotExposer) Expose(ctx context.Context, ownerObject corev1.ObjectReference, param interface{}) error {
	csiExposeParam := param.(*CSISnapshotExposeParam)

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
	})

	curLog.Info("Exposing CSI snapshot")

	volumeSnapshot, err := csi.WaitVolumeSnapshotReady(ctx, e.csiSnapshotClient, csiExposeParam.SnapshotName, csiExposeParam.SourceNamespace, csiExposeParam.Timeout)
	if err != nil {
		return errors.Wrapf(err, "error wait volume snapshot ready")
	}

	curLog.Info("Volumesnapshot is ready")

	vsc, err := csi.GetVolumeSnapshotContentForVolumeSnapshot(volumeSnapshot, e.csiSnapshotClient)
	if err != nil {
		return errors.Wrap(err, "error to get volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).WithField("vs name", volumeSnapshot.Name).Infof("Got VSC from VS in namespace %s", volumeSnapshot.Namespace)

	retained, err := csi.RetainVSC(ctx, e.csiSnapshotClient, vsc)
	if err != nil {
		return errors.Wrap(err, "error to retain volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).WithField("retained", (retained != nil)).Info("Finished to retain VSC")

	defer func() {
		if retained != nil {
			csi.DeleteVolumeSnapshotContentIfAny(ctx, e.csiSnapshotClient, retained.Name, curLog)
		}
	}()

	err = csi.EnsureDeleteVS(ctx, e.csiSnapshotClient, volumeSnapshot.Name, volumeSnapshot.Namespace, csiExposeParam.Timeout)
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot")
	}

	curLog.WithField("vs name", volumeSnapshot.Name).Infof("VS is deleted in namespace %s", volumeSnapshot.Namespace)

	err = csi.EnsureDeleteVSC(ctx, e.csiSnapshotClient, vsc.Name, csiExposeParam.Timeout)
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).Infof("VSC is deleted")
	retained = nil

	backupVS, err := e.createBackupVS(ctx, ownerObject, volumeSnapshot)
	if err != nil {
		return errors.Wrap(err, "error to create backup volume snapshot")
	}

	curLog.WithField("vs name", backupVS.Name).Infof("Backup VS is created from %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)

	defer func() {
		if err != nil {
			csi.DeleteVolumeSnapshotIfAny(ctx, e.csiSnapshotClient, backupVS.Name, backupVS.Namespace, curLog)
		}
	}()

	backupVSC, err := e.createBackupVSC(ctx, ownerObject, vsc, backupVS)
	if err != nil {
		return errors.Wrap(err, "error to create backup volume snapshot content")
	}

	curLog.WithField("vsc name", backupVSC.Name).Infof("Backup VSC is created from %s", vsc.Name)

	backupPVC, err := e.createBackupPVC(ctx, ownerObject, backupVS.Name, csiExposeParam.StorageClass, csiExposeParam.AccessMode, *volumeSnapshot.Status.RestoreSize)
	if err != nil {
		return errors.Wrap(err, "error to create backup pvc")
	}

	curLog.WithField("pvc name", backupPVC.Name).Info("Backup PVC is created")

	defer func() {
		if err != nil {
			kube.DeletePVCIfAny(ctx, e.kubeClient.CoreV1(), backupPVC.Name, backupPVC.Namespace, curLog)
		}
	}()

	backupPod, err := e.createBackupPod(ctx, ownerObject, backupPVC, csiExposeParam.HostingPodLabels)
	if err != nil {
		return errors.Wrap(err, "error to create backup pod")
	}

	curLog.WithField("pod name", backupPod.Name).Info("Backup pod is created")

	defer func() {
		if err != nil {
			kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), backupPod.Name, backupPod.Namespace, curLog)
		}
	}()

	return nil
}

func (e *csiSnapshotExposer) GetExposed(ctx context.Context, ownerObject corev1.ObjectReference, timeout time.Duration, param interface{}) (*ExposeResult, error) {
	exposeWaitParam := param.(*CSISnapshotExposeWaitParam)

	backupPodName := ownerObject.Name
	backupPVCName := ownerObject.Name

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
	})

	pod := &corev1.Pod{}
	err := exposeWaitParam.NodeClient.Get(ctx, types.NamespacedName{
		Namespace: ownerObject.Namespace,
		Name:      backupPodName,
	}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			curLog.WithField("backup pod", backupPodName).Debugf("Backup pod is not running in the current node %s", exposeWaitParam.NodeName)
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "error to get backup pod %s", backupPodName)
		}
	}

	curLog.WithField("pod", pod.Name).Infof("Backup pod is in running state in node %s", pod.Spec.NodeName)

	_, err = kube.WaitPVCBound(ctx, e.kubeClient.CoreV1(), e.kubeClient.CoreV1(), backupPVCName, ownerObject.Namespace, timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "error to wait backup PVC bound, %s", backupPVCName)
	}

	curLog.WithField("backup pvc", backupPVCName).Info("Backup PVC is bound")

	return &ExposeResult{ByPod: ExposeByPod{HostingPod: pod, VolumeName: pod.Spec.Volumes[0].Name}}, nil
}

func (e *csiSnapshotExposer) CleanUp(ctx context.Context, ownerObject corev1.ObjectReference, vsName string, sourceNamespace string) {
	backupPodName := ownerObject.Name
	backupPVCName := ownerObject.Name
	backupVSName := ownerObject.Name

	kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), backupPodName, ownerObject.Namespace, e.log)
	kube.DeletePVCIfAny(ctx, e.kubeClient.CoreV1(), backupPVCName, ownerObject.Namespace, e.log)
	csi.DeleteVolumeSnapshotIfAny(ctx, e.csiSnapshotClient, backupVSName, ownerObject.Namespace, e.log)
	csi.DeleteVolumeSnapshotIfAny(ctx, e.csiSnapshotClient, vsName, sourceNamespace, e.log)
}

func getVolumeModeByAccessMode(accessMode string) (corev1.PersistentVolumeMode, error) {
	if accessMode == AccessModeFileSystem {
		return corev1.PersistentVolumeFilesystem, nil
	} else {
		return "", errors.Errorf("unsupported access mode %s", accessMode)
	}
}

func (e *csiSnapshotExposer) createBackupVS(ctx context.Context, ownerObject corev1.ObjectReference, snapshotVS *snapshotv1api.VolumeSnapshot) (*snapshotv1api.VolumeSnapshot, error) {
	backupVSName := ownerObject.Name
	backupVSCName := ownerObject.Name

	vs := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupVSName,
			Namespace: ownerObject.Namespace,
			// Don't add ownerReference to SnapshotBackup.
			// The backupPVC should be deleted before backupVS, otherwise, the deletion of backupVS will fail since
			// backupPVC has its dataSource referring to it
		},
		Spec: snapshotv1api.VolumeSnapshotSpec{
			Source: snapshotv1api.VolumeSnapshotSource{
				VolumeSnapshotContentName: &backupVSCName,
			},
			VolumeSnapshotClassName: snapshotVS.Spec.VolumeSnapshotClassName,
		},
	}

	return e.csiSnapshotClient.VolumeSnapshots(vs.Namespace).Create(ctx, vs, metav1.CreateOptions{})
}

func (e *csiSnapshotExposer) createBackupVSC(ctx context.Context, ownerObject corev1.ObjectReference, snapshotVSC *snapshotv1api.VolumeSnapshotContent, vs *snapshotv1api.VolumeSnapshot) (*snapshotv1api.VolumeSnapshotContent, error) {
	backupVSCName := ownerObject.Name

	vsc := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupVSCName,
		},
		Spec: snapshotv1api.VolumeSnapshotContentSpec{
			VolumeSnapshotRef: corev1.ObjectReference{
				Name:            vs.Name,
				Namespace:       vs.Namespace,
				UID:             vs.UID,
				ResourceVersion: vs.ResourceVersion,
			},
			Source: snapshotv1api.VolumeSnapshotContentSource{
				SnapshotHandle: snapshotVSC.Status.SnapshotHandle,
			},
			DeletionPolicy:          snapshotVSC.Spec.DeletionPolicy,
			Driver:                  snapshotVSC.Spec.Driver,
			VolumeSnapshotClassName: snapshotVSC.Spec.VolumeSnapshotClassName,
		},
	}

	return e.csiSnapshotClient.VolumeSnapshotContents().Create(ctx, vsc, metav1.CreateOptions{})
}

func (e *csiSnapshotExposer) createBackupPVC(ctx context.Context, ownerObject corev1.ObjectReference, backupVS string, storageClass string, accessMode string, resource resource.Quantity) (*corev1.PersistentVolumeClaim, error) {
	backupVCName := ownerObject.Name

	volumeMode, err := getVolumeModeByAccessMode(accessMode)
	if err != nil {
		return nil, err
	}

	dataSource := &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotv1api.SchemeGroupVersion.Group,
		Kind:     "VolumeSnapshot",
		Name:     backupVS,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ownerObject.Namespace,
			Name:      backupVCName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ownerObject.APIVersion,
					Kind:       ownerObject.Kind,
					Name:       ownerObject.Name,
					UID:        ownerObject.UID,
					Controller: boolptr.True(),
				},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClass,
			VolumeMode:       &volumeMode,
			DataSource:       dataSource,
			DataSourceRef:    dataSource,

			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource,
				},
			},
		},
	}

	created, err := e.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error to create pvc")
	}

	return created, err
}

func (e *csiSnapshotExposer) createBackupPod(ctx context.Context, ownerObject corev1.ObjectReference, backupPVC *corev1.PersistentVolumeClaim, label map[string]string) (*corev1.Pod, error) {
	podName := ownerObject.Name

	volumeName := string(ownerObject.UID)
	containerName := string(ownerObject.UID)

	podInfo, err := getInheritedPodInfo(ctx, e.kubeClient, ownerObject.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "error to get inherited pod info from node-agent")
	}

	var gracePeriod int64 = 0

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ownerObject.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ownerObject.APIVersion,
					Kind:       ownerObject.Kind,
					Name:       ownerObject.Name,
					UID:        ownerObject.UID,
					Controller: boolptr.True(),
				},
			},
			Labels: label,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            containerName,
					Image:           podInfo.image,
					ImagePullPolicy: corev1.PullNever,
					Command:         []string{"/velero-helper", "pause"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      volumeName,
						MountPath: "/" + volumeName,
					}},
				},
			},
			ServiceAccountName:            podInfo.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
			Volumes: []corev1.Volume{{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: backupPVC.Name,
					},
				},
			}},
		},
	}

	return e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}
