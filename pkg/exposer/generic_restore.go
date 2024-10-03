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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// GenericRestoreExposer is the interfaces for a generic restore exposer
type GenericRestoreExposer interface {
	// Expose starts the process to a restore expose, the expose process may take long time
	Expose(context.Context, corev1.ObjectReference, string, string, map[string]string, corev1.ResourceRequirements, time.Duration) error

	// GetExposed polls the status of the expose.
	// If the expose is accessible by the current caller, it waits the expose ready and returns the expose result.
	// Otherwise, it returns nil as the expose result without an error.
	GetExposed(context.Context, corev1.ObjectReference, client.Client, string, time.Duration) (*ExposeResult, error)

	// PeekExposed tests the status of the expose.
	// If the expose is incomplete but not recoverable, it returns an error.
	// Otherwise, it returns nil immediately.
	PeekExposed(context.Context, corev1.ObjectReference) error

	// RebindVolume unexposes the restored PV and rebind it to the target PVC
	RebindVolume(context.Context, corev1.ObjectReference, string, string, time.Duration) error

	// CleanUp cleans up any objects generated during the restore expose
	CleanUp(context.Context, corev1.ObjectReference)
}

// NewGenericRestoreExposer creates a new instance of generic restore exposer
func NewGenericRestoreExposer(kubeClient kubernetes.Interface, log logrus.FieldLogger) GenericRestoreExposer {
	return &genericRestoreExposer{
		kubeClient: kubeClient,
		log:        log,
	}
}

type genericRestoreExposer struct {
	kubeClient kubernetes.Interface
	log        logrus.FieldLogger
}

func (e *genericRestoreExposer) Expose(ctx context.Context, ownerObject corev1.ObjectReference, targetPVCName string, sourceNamespace string, hostingPodLabels map[string]string, resources corev1.ResourceRequirements, timeout time.Duration) error {
	curLog := e.log.WithFields(logrus.Fields{
		"owner":            ownerObject.Name,
		"target PVC":       targetPVCName,
		"source namespace": sourceNamespace,
	})

	selectedNode, targetPVC, err := kube.WaitPVCConsumed(ctx, e.kubeClient.CoreV1(), targetPVCName, sourceNamespace, e.kubeClient.StorageV1(), timeout)
	if err != nil {
		return errors.Wrapf(err, "error to wait target PVC consumed, %s/%s", sourceNamespace, targetPVCName)
	}

	curLog.WithField("target PVC", targetPVCName).WithField("selected node", selectedNode).Info("Target PVC is consumed")

	if kube.IsPVCBound(targetPVC) {
		return errors.Errorf("Target PVC %s/%s has already been bound, abort", sourceNamespace, targetPVCName)
	}

	restorePod, err := e.createRestorePod(ctx, ownerObject, targetPVC, timeout, hostingPodLabels, selectedNode, resources)
	if err != nil {
		return errors.Wrapf(err, "error to create restore pod")
	}

	curLog.WithField("pod name", restorePod.Name).Info("Restore pod is created")

	defer func() {
		if err != nil {
			kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), restorePod.Name, restorePod.Namespace, curLog)
		}
	}()

	restorePVC, err := e.createRestorePVC(ctx, ownerObject, targetPVC, selectedNode)
	if err != nil {
		return errors.Wrap(err, "error to create restore pvc")
	}

	curLog.WithField("pvc name", restorePVC.Name).Info("Restore PVC is created")

	defer func() {
		if err != nil {
			kube.DeletePVAndPVCIfAny(ctx, e.kubeClient.CoreV1(), restorePVC.Name, restorePVC.Namespace, 0, curLog)
		}
	}()

	return nil
}

func (e *genericRestoreExposer) GetExposed(ctx context.Context, ownerObject corev1.ObjectReference, nodeClient client.Client, nodeName string, timeout time.Duration) (*ExposeResult, error) {
	restorePodName := ownerObject.Name
	restorePVCName := ownerObject.Name

	containerName := string(ownerObject.UID)
	volumeName := string(ownerObject.UID)

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
		"node":  nodeName,
	})

	pod := &corev1.Pod{}
	err := nodeClient.Get(ctx, types.NamespacedName{
		Namespace: ownerObject.Namespace,
		Name:      restorePodName,
	}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			curLog.WithField("restore pod", restorePodName).Debug("Restore pod is not running in the current node")
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "error to get restore pod %s", restorePodName)
		}
	}

	curLog.WithField("pod", pod.Name).Infof("Restore pod is in running state in node %s", pod.Spec.NodeName)

	_, err = kube.WaitPVCBound(ctx, e.kubeClient.CoreV1(), e.kubeClient.CoreV1(), restorePVCName, ownerObject.Namespace, timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "error to wait restore PVC bound, %s", restorePVCName)
	}

	curLog.WithField("restore pvc", restorePVCName).Info("Restore PVC is bound")

	i := 0
	for i = 0; i < len(pod.Spec.Volumes); i++ {
		if pod.Spec.Volumes[i].Name == volumeName {
			break
		}
	}

	if i == len(pod.Spec.Volumes) {
		return nil, errors.Errorf("restore pod %s doesn't have the expected restore volume", pod.Name)
	}

	curLog.WithField("pod", pod.Name).Infof("Restore volume is found in pod at index %v", i)

	return &ExposeResult{ByPod: ExposeByPod{
		HostingPod:       pod,
		HostingContainer: containerName,
		VolumeName:       volumeName,
	}}, nil
}

func (e *genericRestoreExposer) PeekExposed(ctx context.Context, ownerObject corev1.ObjectReference) error {
	restorePodName := ownerObject.Name

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
	})

	pod, err := e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(ctx, restorePodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		curLog.WithError(err).Warnf("error to peek restore pod %s", restorePodName)
		return nil
	}

	if podFailed, message := kube.IsPodUnrecoverable(pod, curLog); podFailed {
		return errors.New(message)
	}

	return nil
}

func (e *genericRestoreExposer) CleanUp(ctx context.Context, ownerObject corev1.ObjectReference) {
	restorePodName := ownerObject.Name
	restorePVCName := ownerObject.Name

	kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), restorePodName, ownerObject.Namespace, e.log)
	kube.DeletePVAndPVCIfAny(ctx, e.kubeClient.CoreV1(), restorePVCName, ownerObject.Namespace, 0, e.log)
}

func (e *genericRestoreExposer) RebindVolume(ctx context.Context, ownerObject corev1.ObjectReference, targetPVCName string, sourceNamespace string, timeout time.Duration) error {
	restorePodName := ownerObject.Name
	restorePVCName := ownerObject.Name

	curLog := e.log.WithFields(logrus.Fields{
		"owner":            ownerObject.Name,
		"target PVC":       targetPVCName,
		"source namespace": sourceNamespace,
	})

	targetPVC, err := e.kubeClient.CoreV1().PersistentVolumeClaims(sourceNamespace).Get(ctx, targetPVCName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "error to get target PVC %s/%s", sourceNamespace, targetPVCName)
	}

	restorePV, err := kube.WaitPVCBound(ctx, e.kubeClient.CoreV1(), e.kubeClient.CoreV1(), restorePVCName, ownerObject.Namespace, timeout)
	if err != nil {
		return errors.Wrapf(err, "error to get PV from restore PVC %s", restorePVCName)
	}

	orgReclaim := restorePV.Spec.PersistentVolumeReclaimPolicy

	curLog.WithField("restore PV", restorePV.Name).Info("Restore PV is retrieved")

	retained, err := kube.SetPVReclaimPolicy(ctx, e.kubeClient.CoreV1(), restorePV, corev1.PersistentVolumeReclaimRetain)
	if err != nil {
		return errors.Wrapf(err, "error to retain PV %s", restorePV.Name)
	}

	curLog.WithField("restore PV", restorePV.Name).WithField("retained", (retained != nil)).Info("Restore PV is retained")

	defer func() {
		if retained != nil {
			curLog.WithField("retained PV", retained.Name).Info("Deleting retained PV on error")
			kube.DeletePVIfAny(ctx, e.kubeClient.CoreV1(), retained.Name, curLog)
		}
	}()

	if retained != nil {
		restorePV = retained
	}

	err = kube.EnsureDeletePod(ctx, e.kubeClient.CoreV1(), restorePodName, ownerObject.Namespace, timeout)
	if err != nil {
		return errors.Wrapf(err, "error to delete restore pod %s", restorePodName)
	}

	err = kube.EnsureDeletePVC(ctx, e.kubeClient.CoreV1(), restorePVCName, ownerObject.Namespace, timeout)
	if err != nil {
		return errors.Wrapf(err, "error to delete restore PVC %s", restorePVCName)
	}

	curLog.WithField("restore PVC", restorePVCName).Info("Restore PVC is deleted")

	_, err = kube.RebindPVC(ctx, e.kubeClient.CoreV1(), targetPVC, restorePV.Name)
	if err != nil {
		return errors.Wrapf(err, "error to rebind target PVC %s/%s to %s", targetPVC.Namespace, targetPVC.Name, restorePV.Name)
	}

	curLog.WithField("tartet PVC", fmt.Sprintf("%s/%s", targetPVC.Namespace, targetPVC.Name)).WithField("restore PV", restorePV.Name).Info("Target PVC is rebound to restore PV")

	var matchLabel map[string]string
	if targetPVC.Spec.Selector != nil {
		matchLabel = targetPVC.Spec.Selector.MatchLabels
	}

	restorePVName := restorePV.Name
	restorePV, err = kube.ResetPVBinding(ctx, e.kubeClient.CoreV1(), restorePV, matchLabel, targetPVC)
	if err != nil {
		return errors.Wrapf(err, "error to reset binding info for restore PV %s", restorePVName)
	}

	curLog.WithField("restore PV", restorePV.Name).Info("Restore PV is rebound")

	restorePV, err = kube.WaitPVBound(ctx, e.kubeClient.CoreV1(), restorePV.Name, targetPVC.Name, targetPVC.Namespace, timeout)
	if err != nil {
		return errors.Wrapf(err, "error to wait restore PV bound, restore PV %s", restorePVName)
	}

	curLog.WithField("restore PV", restorePV.Name).Info("Restore PV is ready")

	retained = nil

	_, err = kube.SetPVReclaimPolicy(ctx, e.kubeClient.CoreV1(), restorePV, orgReclaim)
	if err != nil {
		curLog.WithField("restore PV", restorePV.Name).WithError(err).Warn("Restore PV's reclaim policy is not restored")
	} else {
		curLog.WithField("restore PV", restorePV.Name).Info("Restore PV's reclaim policy is restored")
	}

	return nil
}

func (e *genericRestoreExposer) createRestorePod(ctx context.Context, ownerObject corev1.ObjectReference, targetPVC *corev1.PersistentVolumeClaim,
	operationTimeout time.Duration, label map[string]string, selectedNode string, resources corev1.ResourceRequirements) (*corev1.Pod, error) {
	restorePodName := ownerObject.Name
	restorePVCName := ownerObject.Name

	containerName := string(ownerObject.UID)
	volumeName := string(ownerObject.UID)

	podInfo, err := getInheritedPodInfo(ctx, e.kubeClient, ownerObject.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "error to get inherited pod info from node-agent")
	}

	var gracePeriod int64 = 0
	volumeMounts, volumeDevices, volumePath := kube.MakePodPVCAttachment(volumeName, targetPVC.Spec.VolumeMode, false)
	volumeMounts = append(volumeMounts, podInfo.volumeMounts...)

	volumes := []corev1.Volume{{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: restorePVCName,
			},
		},
	}}
	volumes = append(volumes, podInfo.volumes...)

	volumeMode := corev1.PersistentVolumeFilesystem
	if targetPVC.Spec.VolumeMode != nil {
		volumeMode = *targetPVC.Spec.VolumeMode
	}

	args := []string{
		fmt.Sprintf("--volume-path=%s", volumePath),
		fmt.Sprintf("--volume-mode=%s", volumeMode),
		fmt.Sprintf("--data-download=%s", ownerObject.Name),
		fmt.Sprintf("--resource-timeout=%s", operationTimeout.String()),
	}

	args = append(args, podInfo.logFormatArgs...)
	args = append(args, podInfo.logLevelArgs...)

	userID := int64(0)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restorePodName,
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
					Command: []string{
						"/velero",
						"data-mover",
						"restore",
					},
					Args:          args,
					VolumeMounts:  volumeMounts,
					VolumeDevices: volumeDevices,
					Env:           podInfo.env,
					Resources:     resources,
				},
			},
			ServiceAccountName:            podInfo.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
			Volumes:                       volumes,
			NodeName:                      selectedNode,
			RestartPolicy:                 corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &userID,
			},
		},
	}

	return e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func (e *genericRestoreExposer) createRestorePVC(ctx context.Context, ownerObject corev1.ObjectReference, targetPVC *corev1.PersistentVolumeClaim, selectedNode string) (*corev1.PersistentVolumeClaim, error) {
	restorePVCName := ownerObject.Name

	pvcObj := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ownerObject.Namespace,
			Name:        restorePVCName,
			Labels:      targetPVC.Labels,
			Annotations: targetPVC.Annotations,
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
			AccessModes:      targetPVC.Spec.AccessModes,
			StorageClassName: targetPVC.Spec.StorageClassName,
			VolumeMode:       targetPVC.Spec.VolumeMode,
			Resources:        targetPVC.Spec.Resources,
		},
	}

	if selectedNode != "" {
		pvcObj.Annotations = map[string]string{
			kube.KubeAnnSelectedNode: selectedNode,
		}
	}

	return e.kubeClient.CoreV1().PersistentVolumeClaims(pvcObj.Namespace).Create(ctx, pvcObj, metav1.CreateOptions{})
}
