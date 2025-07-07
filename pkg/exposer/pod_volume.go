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
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	PodVolumeExposeTypeBackup  = "pod-volume-backup"
	PodVolumeExposeTypeRestore = "pod-volume-restore"
)

// PodVolumeExposeParam define the input param for pod volume Expose
type PodVolumeExposeParam struct {
	// ClientPodName is the name of pod to be backed up or restored
	ClientPodName string

	// ClientNamespace is the namespace to be backed up or restored
	ClientNamespace string

	// ClientNamespace is the pod volume for the client PVC
	ClientPodVolume string

	// HostingPodLabels is the labels that are going to apply to the hosting pod
	HostingPodLabels map[string]string

	// HostingPodAnnotations is the annotations that are going to apply to the hosting pod
	HostingPodAnnotations map[string]string

	// HostingPodTolerations is the tolerations that are going to apply to the hosting pod
	HostingPodTolerations []corev1api.Toleration

	// Resources defines the resource requirements of the hosting pod
	Resources corev1api.ResourceRequirements

	// OperationTimeout specifies the time wait for resources operations in Expose
	OperationTimeout time.Duration

	// Type specifies the type of the expose, either backup or erstore
	Type string
}

// PodVolumeExposer is the interfaces for a pod volume exposer
type PodVolumeExposer interface {
	// Expose starts the process to a pod volume expose, the expose process may take long time
	Expose(context.Context, corev1api.ObjectReference, PodVolumeExposeParam) error

	// GetExposed polls the status of the expose.
	// If the expose is accessible by the current caller, it waits the expose ready and returns the expose result.
	// Otherwise, it returns nil as the expose result without an error.
	GetExposed(context.Context, corev1api.ObjectReference, client.Client, string, time.Duration) (*ExposeResult, error)

	// PeekExposed tests the status of the expose.
	// If the expose is incomplete but not recoverable, it returns an error.
	// Otherwise, it returns nil immediately.
	PeekExposed(context.Context, corev1api.ObjectReference) error

	// DiagnoseExpose generate the diagnostic info when the expose is not finished for a long time.
	// If it finds any problem, it returns an string about the problem.
	DiagnoseExpose(context.Context, corev1api.ObjectReference) string

	// CleanUp cleans up any objects generated during the restore expose
	CleanUp(context.Context, corev1api.ObjectReference)
}

// NewPodVolumeExposer creates a new instance of pod volume exposer
func NewPodVolumeExposer(kubeClient kubernetes.Interface, log logrus.FieldLogger) PodVolumeExposer {
	return &podVolumeExposer{
		kubeClient: kubeClient,
		fs:         filesystem.NewFileSystem(),
		log:        log,
	}
}

type podVolumeExposer struct {
	kubeClient kubernetes.Interface
	fs         filesystem.Interface
	log        logrus.FieldLogger
}

var getPodVolumeHostPath = GetPodVolumeHostPath
var extractPodVolumeHostPath = ExtractPodVolumeHostPath

func (e *podVolumeExposer) Expose(ctx context.Context, ownerObject corev1api.ObjectReference, param PodVolumeExposeParam) error {
	curLog := e.log.WithFields(logrus.Fields{
		"owner":             ownerObject.Name,
		"client pod":        param.ClientPodName,
		"client pod volume": param.ClientPodVolume,
		"client namespace":  param.ClientNamespace,
		"type":              param.Type,
	})

	pod, err := e.kubeClient.CoreV1().Pods(param.ClientNamespace).Get(ctx, param.ClientPodName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "error getting client pod %s", param.ClientPodName)
	}

	if pod.Spec.NodeName == "" {
		return errors.Errorf("client pod %s doesn't have a node name", pod.Name)
	}

	nodeOS, err := kube.GetNodeOS(ctx, pod.Spec.NodeName, e.kubeClient.CoreV1())
	if err != nil {
		return errors.Wrapf(err, "error getting OS for node %s", pod.Spec.NodeName)
	}

	curLog.Infof("Client pod is running in node %s, os %s", pod.Spec.NodeName, nodeOS)

	path, err := getPodVolumeHostPath(ctx, pod, param.ClientPodVolume, e.kubeClient, e.fs, e.log)
	if err != nil {
		return errors.Wrapf(err, "error to get pod volume path")
	}

	path.ByPath, err = extractPodVolumeHostPath(ctx, path.ByPath, e.kubeClient, ownerObject.Namespace, nodeOS)
	if err != nil {
		return errors.Wrapf(err, "error to extract pod volume path")
	}

	curLog.WithField("path", path).Infof("Host path is retrieved for pod %s, volume %s", param.ClientPodName, param.ClientPodVolume)

	hostingPod, err := e.createHostingPod(ctx, ownerObject, param.Type, path.ByPath, param.OperationTimeout, param.HostingPodLabels, param.HostingPodAnnotations, param.HostingPodTolerations, pod.Spec.NodeName, param.Resources, nodeOS)
	if err != nil {
		return errors.Wrapf(err, "error to create hosting pod")
	}

	curLog.WithField("pod name", hostingPod.Name).Info("Hosting pod is created")

	return nil
}

func (e *podVolumeExposer) GetExposed(ctx context.Context, ownerObject corev1api.ObjectReference, nodeClient client.Client, nodeName string, timeout time.Duration) (*ExposeResult, error) {
	hostingPodName := ownerObject.Name

	containerName := string(ownerObject.UID)
	volumeName := string(ownerObject.UID)

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
		"node":  nodeName,
	})

	var updated *corev1api.Pod
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod := &corev1api.Pod{}
		err := nodeClient.Get(ctx, types.NamespacedName{
			Namespace: ownerObject.Namespace,
			Name:      hostingPodName,
		}, pod)

		if err != nil {
			return false, errors.Wrapf(err, "error to get pod %s/%s", ownerObject.Namespace, hostingPodName)
		}

		if pod.Status.Phase != corev1api.PodRunning {
			return false, nil
		}

		updated = pod

		return true, nil
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			curLog.WithField("hosting pod", hostingPodName).Debug("Hosting pod is not running in the current node")
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "error to wait for rediness of pod %s", hostingPodName)
		}
	}

	curLog.WithField("pod", updated.Name).Infof("Hosting pod is in running state in node %s", updated.Spec.NodeName)

	return &ExposeResult{ByPod: ExposeByPod{
		HostingPod:       updated,
		HostingContainer: containerName,
		VolumeName:       volumeName,
	}}, nil
}

func (e *podVolumeExposer) PeekExposed(ctx context.Context, ownerObject corev1api.ObjectReference) error {
	hostingPodName := ownerObject.Name

	curLog := e.log.WithFields(logrus.Fields{
		"owner": ownerObject.Name,
	})

	pod, err := e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(ctx, hostingPodName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		curLog.WithError(err).Warnf("error to peek hosting pod %s", hostingPodName)
		return nil
	}

	if podFailed, message := kube.IsPodUnrecoverable(pod, curLog); podFailed {
		return errors.New(message)
	}

	return nil
}

func (e *podVolumeExposer) DiagnoseExpose(ctx context.Context, ownerObject corev1api.ObjectReference) string {
	hostingPodName := ownerObject.Name

	diag := "begin diagnose pod volume exposer\n"

	pod, err := e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(ctx, hostingPodName, metav1.GetOptions{})
	if err != nil {
		pod = nil
		diag += fmt.Sprintf("error getting hosting pod %s, err: %v\n", hostingPodName, err)
	}

	if pod != nil {
		diag += kube.DiagnosePod(pod)

		if pod.Spec.NodeName != "" {
			if err := nodeagent.KbClientIsRunningInNode(ctx, ownerObject.Namespace, pod.Spec.NodeName, e.kubeClient); err != nil {
				diag += fmt.Sprintf("node-agent is not running in node %s, err: %v\n", pod.Spec.NodeName, err)
			}
		}
	}

	diag += "end diagnose pod volume exposer"

	return diag
}

func (e *podVolumeExposer) CleanUp(ctx context.Context, ownerObject corev1api.ObjectReference) {
	restorePodName := ownerObject.Name
	kube.DeletePodIfAny(ctx, e.kubeClient.CoreV1(), restorePodName, ownerObject.Namespace, e.log)
}

func (e *podVolumeExposer) createHostingPod(ctx context.Context, ownerObject corev1api.ObjectReference, exposeType string, hostPath string,
	operationTimeout time.Duration, label map[string]string, annotation map[string]string, toleration []corev1api.Toleration, selectedNode string, resources corev1api.ResourceRequirements, nodeOS string) (*corev1api.Pod, error) {
	hostingPodName := ownerObject.Name

	containerName := string(ownerObject.UID)
	clientVolumeName := string(ownerObject.UID)
	clientVolumePath := "/" + clientVolumeName

	podInfo, err := getInheritedPodInfo(ctx, e.kubeClient, ownerObject.Namespace, nodeOS)
	if err != nil {
		return nil, errors.Wrap(err, "error to get inherited pod info from node-agent")
	}

	var gracePeriod int64
	mountPropagation := corev1api.MountPropagationHostToContainer
	volumeMounts := []corev1api.VolumeMount{{
		Name:             clientVolumeName,
		MountPath:        clientVolumePath,
		MountPropagation: &mountPropagation,
	}}
	volumeMounts = append(volumeMounts, podInfo.volumeMounts...)

	volumes := []corev1api.Volume{{
		Name: clientVolumeName,
		VolumeSource: corev1api.VolumeSource{
			HostPath: &corev1api.HostPathVolumeSource{
				Path: hostPath,
			},
		},
	}}
	volumes = append(volumes, podInfo.volumes...)

	args := []string{
		fmt.Sprintf("--volume-path=%s", clientVolumePath),
		fmt.Sprintf("--resource-timeout=%s", operationTimeout.String()),
	}

	command := []string{
		"/velero",
		"pod-volume",
	}

	if exposeType == PodVolumeExposeTypeBackup {
		args = append(args, fmt.Sprintf("--pod-volume-backup=%s", ownerObject.Name))
		command = append(command, "backup")
	} else {
		args = append(args, fmt.Sprintf("--pod-volume-restore=%s", ownerObject.Name))
		command = append(command, "restore")
	}

	args = append(args, podInfo.logFormatArgs...)
	args = append(args, podInfo.logLevelArgs...)

	var securityCtx *corev1api.PodSecurityContext
	nodeSelector := map[string]string{}
	podOS := corev1api.PodOS{}
	if nodeOS == kube.NodeOSWindows {
		userID := "ContainerAdministrator"
		securityCtx = &corev1api.PodSecurityContext{
			WindowsOptions: &corev1api.WindowsSecurityContextOptions{
				RunAsUserName: &userID,
			},
		}

		nodeSelector[kube.NodeOSLabel] = kube.NodeOSWindows
		podOS.Name = kube.NodeOSWindows

		toleration = append(toleration, corev1api.Toleration{
			Key:      "os",
			Operator: "Equal",
			Effect:   "NoSchedule",
			Value:    "windows",
		})
	} else {
		userID := int64(0)
		securityCtx = &corev1api.PodSecurityContext{
			RunAsUser: &userID,
		}

		nodeSelector[kube.NodeOSLabel] = kube.NodeOSLinux
		podOS.Name = kube.NodeOSLinux
	}

	pod := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostingPodName,
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
			Labels:      label,
			Annotations: annotation,
		},
		Spec: corev1api.PodSpec{
			NodeSelector: nodeSelector,
			OS:           &podOS,
			Containers: []corev1api.Container{
				{
					Name:            containerName,
					Image:           podInfo.image,
					ImagePullPolicy: corev1api.PullNever,
					Command:         command,
					Args:            args,
					VolumeMounts:    volumeMounts,
					Env:             podInfo.env,
					EnvFrom:         podInfo.envFrom,
					Resources:       resources,
				},
			},
			ServiceAccountName:            podInfo.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
			Volumes:                       volumes,
			NodeName:                      selectedNode,
			RestartPolicy:                 corev1api.RestartPolicyNever,
			SecurityContext:               securityCtx,
			Tolerations:                   toleration,
		},
	}

	return e.kubeClient.CoreV1().Pods(ownerObject.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}
