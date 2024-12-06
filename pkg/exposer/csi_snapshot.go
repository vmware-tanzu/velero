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
	"fmt"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	snapshotter "github.com/kubernetes-csi/external-snapshotter/client/v7/clientset/versioned/typed/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/csi"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
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

	// OperationTimeout specifies the time wait for resources operations in Expose
	OperationTimeout time.Duration

	// ExposeTimeout specifies the timeout for the entire expose process
	ExposeTimeout time.Duration

	// VolumeSize specifies the size of the source volume
	VolumeSize resource.Quantity

	// Affinity specifies the node affinity of the backup pod
	Affinity *kube.LoadAffinity

	// BackupPVCConfig is the config for backupPVC (intermediate PVC) of snapshot data movement
	BackupPVCConfig map[string]nodeagent.BackupPVC

	// Resources defines the resource requirements of the hosting pod
	Resources corev1.ResourceRequirements
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

	volumeSnapshot, err := csi.WaitVolumeSnapshotReady(ctx, e.csiSnapshotClient, csiExposeParam.SnapshotName, csiExposeParam.SourceNamespace, csiExposeParam.ExposeTimeout, curLog)
	if err != nil {
		return errors.Wrapf(err, "error wait volume snapshot ready")
	}

	curLog.Info("Volumesnapshot is ready")

	vsc, err := csi.GetVolumeSnapshotContentForVolumeSnapshot(volumeSnapshot, e.csiSnapshotClient)
	if err != nil {
		return errors.Wrap(err, "error to get volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).WithField("vs name", volumeSnapshot.Name).Infof("Got VSC from VS in namespace %s", volumeSnapshot.Namespace)

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

	retained, err := csi.RetainVSC(ctx, e.csiSnapshotClient, vsc)
	if err != nil {
		return errors.Wrap(err, "error to retain volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).WithField("retained", (retained != nil)).Info("Finished to retain VSC")

	err = csi.EnsureDeleteVS(ctx, e.csiSnapshotClient, volumeSnapshot.Name, volumeSnapshot.Namespace, csiExposeParam.OperationTimeout)
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot")
	}

	curLog.WithField("vs name", volumeSnapshot.Name).Infof("VS is deleted in namespace %s", volumeSnapshot.Namespace)

	err = csi.EnsureDeleteVSC(ctx, e.csiSnapshotClient, vsc.Name, csiExposeParam.OperationTimeout)
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot content")
	}

	curLog.WithField("vsc name", vsc.Name).Infof("VSC is deleted")

	var volumeSize resource.Quantity
	if volumeSnapshot.Status.RestoreSize != nil && !volumeSnapshot.Status.RestoreSize.IsZero() {
		volumeSize = *volumeSnapshot.Status.RestoreSize
	} else {
		volumeSize = csiExposeParam.VolumeSize
		curLog.WithField("vs name", volumeSnapshot.Name).Warnf("The snapshot doesn't contain a valid restore size, use source volume's size %v", volumeSize)
	}

	// check if there is a mapping for source pvc storage class in backupPVC config
	// if the mapping exists then use the values(storage class, readOnly accessMode)
	// for backupPVC (intermediate PVC in snapshot data movement) object creation
	backupPVCStorageClass := csiExposeParam.StorageClass
	backupPVCReadOnly := false
	spcNoRelabeling := false
	if value, exists := csiExposeParam.BackupPVCConfig[csiExposeParam.StorageClass]; exists {
		if value.StorageClass != "" {
			backupPVCStorageClass = value.StorageClass
		}

		backupPVCReadOnly = value.ReadOnly
		if value.SPCNoRelabeling {
			if backupPVCReadOnly {
				spcNoRelabeling = true
			} else {
				curLog.WithField("vs name", volumeSnapshot.Name).Warn("Ignoring spcNoRelabling for read-write volume")
			}
		}
	}

	backupPVC, err := e.createBackupPVC(ctx, ownerObject, backupVS.Name, backupPVCStorageClass, csiExposeParam.AccessMode, volumeSize, backupPVCReadOnly)
	if err != nil {
		return errors.Wrap(err, "error to create backup pvc")
	}

	curLog.WithField("pvc name", backupPVC.Name).Info("Backup PVC is created")
	defer func() {
		if err != nil {
			kube.DeletePVAndPVCIfAny(ctx, e.kubeClient.CoreV1(), backupPVC.Name, backupPVC.Namespace, 0, curLog)
		}
	}()

	backupPod, err := e.createBackupPod(
		ctx,
		ownerObject,
		backupPVC,
		csiExposeParam.OperationTimeout,
		csiExposeParam.HostingPodLabels,
		csiExposeParam.Affinity,
		csiExposeParam.Resources,
		backupPVCReadOnly,
		spcNoRelabeling,
	)
	if err != nil {
		return errors.Wrap(err, "error to create backup pod")
	}

	curLog.WithField("pod name", backupPod.Name).WithField("affinity", csiExposeParam.Affinity).Info("Backup pod is created")

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

	containerName := string(ownerObject.UID)
	volumeName := string(ownerObject.UID)

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

	i := 0
	for i = 0; i < len(pod.Spec.Volumes); i++ {
		if pod.Spec.Volumes[i].Name == volumeName {
			break
		}
	}

	if i == len(pod.Spec.Volumes) {
		return nil, errors.Errorf("backup pod %s doesn't have the expected backup volume", pod.Name)
	}

	curLog.WithField("pod", pod.Name).Infof("Backup volume is found in pod at index %v", i)

	return &ExposeResult{ByPod: ExposeByPod{
		HostingPod:       pod,
		HostingContainer: containerName,
		VolumeName:       volumeName,
	}}, nil
}

func (e *csiSnapshotExposer) PeekExposed(ctx context.Context, ownerObject corev1.ObjectReference) error {
	backupPodName := ownerObject.Name

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
	})

	pod, err := e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(ctx, backupPodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		curLog.WithError(err).Warnf("error to peek backup pod %s", backupPodName)
		return nil
	}

	if podFailed, message := kube.IsPodUnrecoverable(pod, curLog); podFailed {
		return errors.New(message)
	}

	return nil
}

const cleanUpTimeout = time.Minute

func (e *csiSnapshotExposer) CleanUp(ctx context.Context, ownerObject corev1.ObjectReference, vsName string, sourceNamespace string) {
	backupPodName := ownerObject.Name
	backupPVCName := ownerObject.Name
	backupVSName := ownerObject.Name

	kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), backupPodName, ownerObject.Namespace, e.log)
	kube.DeletePVAndPVCIfAny(ctx, e.kubeClient.CoreV1(), backupPVCName, ownerObject.Namespace, cleanUpTimeout, e.log)

	csi.DeleteVolumeSnapshotIfAny(ctx, e.csiSnapshotClient, backupVSName, ownerObject.Namespace, e.log)
	csi.DeleteVolumeSnapshotIfAny(ctx, e.csiSnapshotClient, vsName, sourceNamespace, e.log)
}

func getVolumeModeByAccessMode(accessMode string) (corev1.PersistentVolumeMode, error) {
	switch accessMode {
	case AccessModeFileSystem:
		return corev1.PersistentVolumeFilesystem, nil
	case AccessModeBlock:
		return corev1.PersistentVolumeBlock, nil
	default:
		return "", errors.Errorf("unsupported access mode %s", accessMode)
	}
}

func (e *csiSnapshotExposer) createBackupVS(ctx context.Context, ownerObject corev1.ObjectReference, snapshotVS *snapshotv1api.VolumeSnapshot) (*snapshotv1api.VolumeSnapshot, error) {
	backupVSName := ownerObject.Name
	backupVSCName := ownerObject.Name

	vs := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:        backupVSName,
			Namespace:   ownerObject.Namespace,
			Annotations: snapshotVS.Annotations,
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
			Name:        backupVSCName,
			Annotations: snapshotVSC.Annotations,
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
			DeletionPolicy:          snapshotv1api.VolumeSnapshotContentDelete,
			Driver:                  snapshotVSC.Spec.Driver,
			VolumeSnapshotClassName: snapshotVSC.Spec.VolumeSnapshotClassName,
		},
	}

	return e.csiSnapshotClient.VolumeSnapshotContents().Create(ctx, vsc, metav1.CreateOptions{})
}

func (e *csiSnapshotExposer) createBackupPVC(ctx context.Context, ownerObject corev1.ObjectReference, backupVS, storageClass, accessMode string, resource resource.Quantity, readOnly bool) (*corev1.PersistentVolumeClaim, error) {
	backupPVCName := ownerObject.Name

	volumeMode, err := getVolumeModeByAccessMode(accessMode)
	if err != nil {
		return nil, err
	}

	pvcAccessMode := corev1.ReadWriteOnce

	if readOnly {
		pvcAccessMode = corev1.ReadOnlyMany
	}

	dataSource := &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotv1api.SchemeGroupVersion.Group,
		Kind:     "VolumeSnapshot",
		Name:     backupVS,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ownerObject.Namespace,
			Name:      backupPVCName,
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
				pvcAccessMode,
			},
			StorageClassName: &storageClass,
			VolumeMode:       &volumeMode,
			DataSource:       dataSource,
			DataSourceRef:    nil,

			Resources: corev1.VolumeResourceRequirements{
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

func (e *csiSnapshotExposer) createBackupPod(
	ctx context.Context,
	ownerObject corev1.ObjectReference,
	backupPVC *corev1.PersistentVolumeClaim,
	operationTimeout time.Duration,
	label map[string]string,
	affinity *kube.LoadAffinity,
	resources corev1.ResourceRequirements,
	backupPVCReadOnly bool,
	spcNoRelabeling bool,
) (*corev1.Pod, error) {
	podName := ownerObject.Name

	containerName := string(ownerObject.UID)
	volumeName := string(ownerObject.UID)

	podInfo, err := getInheritedPodInfo(ctx, e.kubeClient, ownerObject.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "error to get inherited pod info from node-agent")
	}

	var gracePeriod int64 = 0
	volumeMounts, volumeDevices, volumePath := kube.MakePodPVCAttachment(volumeName, backupPVC.Spec.VolumeMode, backupPVCReadOnly)
	volumeMounts = append(volumeMounts, podInfo.volumeMounts...)

	volumes := []corev1.Volume{{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: backupPVC.Name,
			},
		},
	}}

	if backupPVCReadOnly {
		volumes[0].VolumeSource.PersistentVolumeClaim.ReadOnly = true
	}

	volumes = append(volumes, podInfo.volumes...)

	if label == nil {
		label = make(map[string]string)
	}
	label[podGroupLabel] = podGroupSnapshot

	volumeMode := corev1.PersistentVolumeFilesystem
	if backupPVC.Spec.VolumeMode != nil {
		volumeMode = *backupPVC.Spec.VolumeMode
	}

	args := []string{
		fmt.Sprintf("--volume-path=%s", volumePath),
		fmt.Sprintf("--volume-mode=%s", volumeMode),
		fmt.Sprintf("--data-upload=%s", ownerObject.Name),
		fmt.Sprintf("--resource-timeout=%s", operationTimeout.String()),
	}

	args = append(args, podInfo.logFormatArgs...)
	args = append(args, podInfo.logLevelArgs...)

	userID := int64(0)

	affinityList := make([]*kube.LoadAffinity, 0)
	if affinity != nil {
		affinityList = append(affinityList, affinity)
	}

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
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{
					MaxSkew:           1,
					TopologyKey:       "kubernetes.io/hostname",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							podGroupLabel: podGroupSnapshot,
						},
					},
				},
			},
			Affinity: kube.ToSystemAffinity(affinityList),
			Containers: []corev1.Container{
				{
					Name:            containerName,
					Image:           podInfo.image,
					ImagePullPolicy: corev1.PullNever,
					Command: []string{
						"/velero",
						"data-mover",
						"backup",
					},
					Args:          args,
					VolumeMounts:  volumeMounts,
					VolumeDevices: volumeDevices,
					Env:           podInfo.env,
					EnvFrom:       podInfo.envFrom,
					Resources:     resources,
				},
			},
			ServiceAccountName:            podInfo.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
			Volumes:                       volumes,
			RestartPolicy:                 corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &userID,
			},
		},
	}

	if spcNoRelabeling {
		pod.Spec.SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{
			Type: "spc_t",
		}
	}

	return e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}
