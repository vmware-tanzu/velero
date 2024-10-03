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
package kube

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type LoadAffinity struct {
	// NodeSelector specifies the label selector to match nodes
	NodeSelector metav1.LabelSelector `json:"nodeSelector"`
}

type PodResources struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// IsPodRunning does a well-rounded check to make sure the specified pod is running stably.
// If not, return the error found
func IsPodRunning(pod *corev1api.Pod) error {
	return isPodScheduledInStatus(pod, func(pod *corev1api.Pod) error {
		if pod.Status.Phase != corev1api.PodRunning {
			return errors.New("pod is not running")
		}

		return nil
	})
}

// IsPodScheduled does a well-rounded check to make sure the specified pod has been scheduled into a node and in a stable status.
// If not, return the error found
func IsPodScheduled(pod *corev1api.Pod) error {
	return isPodScheduledInStatus(pod, func(pod *corev1api.Pod) error {
		if pod.Status.Phase != corev1api.PodRunning && pod.Status.Phase != corev1api.PodPending {
			return errors.New("pod is not running or pending")
		}

		return nil
	})
}

func isPodScheduledInStatus(pod *corev1api.Pod, statusCheckFunc func(*corev1api.Pod) error) error {
	if pod == nil {
		return errors.New("invalid input pod")
	}

	if pod.Spec.NodeName == "" {
		return errors.Errorf("pod is not scheduled, name=%s, namespace=%s, phase=%s", pod.Name, pod.Namespace, pod.Status.Phase)
	}

	if err := statusCheckFunc(pod); err != nil {
		return errors.Wrapf(err, "pod is not in the expected status, name=%s, namespace=%s, phase=%s", pod.Name, pod.Namespace, pod.Status.Phase)
	}

	if pod.DeletionTimestamp != nil {
		return errors.Errorf("pod is being terminated, name=%s, namespace=%s, phase=%s", pod.Name, pod.Namespace, pod.Status.Phase)
	}

	return nil
}

// DeletePodIfAny deletes a pod by name if it exists, and log an error when the deletion fails
func DeletePodIfAny(ctx context.Context, podGetter corev1client.CoreV1Interface, podName string, podNamespace string, log logrus.FieldLogger) {
	err := podGetter.Pods(podNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debugf("Abort deleting pod, it doesn't exist %s/%s", podNamespace, podName)
		} else {
			log.WithError(err).Errorf("Failed to delete pod %s/%s", podNamespace, podName)
		}
	}
}

// EnsureDeletePod asserts the existence of a pod by name, deletes it and waits for its disappearance and returns errors on any failure
func EnsureDeletePod(ctx context.Context, podGetter corev1client.CoreV1Interface, pod string, namespace string, timeout time.Duration) error {
	err := podGetter.Pods(namespace).Delete(ctx, pod, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "error to delete pod %s", pod)
	}

	err = wait.PollUntilContextTimeout(ctx, waitInternal, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := podGetter.Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, errors.Wrapf(err, "error to get pod %s", pod)
		}

		return false, nil
	})

	if err != nil {
		return errors.Wrapf(err, "error to assure pod is deleted, %s", pod)
	}

	return nil
}

// IsPodUnrecoverable checks if the pod is in an abnormal state and could not be recovered
// It could not cover all the cases but we could add more cases in the future
func IsPodUnrecoverable(pod *corev1api.Pod, log logrus.FieldLogger) (bool, string) {
	// Check the Phase field
	if pod.Status.Phase == corev1api.PodFailed || pod.Status.Phase == corev1api.PodUnknown {
		message := GetPodTerminateMessage(pod)
		log.Warnf("Pod is in abnormal state %s, message [%s]", pod.Status.Phase, message)
		return true, fmt.Sprintf("Pod is in abnormal state [%s], message [%s]", pod.Status.Phase, message)
	}

	// removed "Unschedulable" check since unschedulable condition isn't always permanent

	// Check the Status field
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// If the container's image state is ImagePullBackOff, it indicates an image pull failure
		if containerStatus.State.Waiting != nil && (containerStatus.State.Waiting.Reason == "ImagePullBackOff" || containerStatus.State.Waiting.Reason == "ErrImageNeverPull") {
			log.Warnf("Container %s in Pod %s/%s is in pull image failed with reason %s", containerStatus.Name, pod.Namespace, pod.Name, containerStatus.State.Waiting.Reason)
			return true, fmt.Sprintf("Container %s in Pod %s/%s is in pull image failed with reason %s", containerStatus.Name, pod.Namespace, pod.Name, containerStatus.State.Waiting.Reason)
		}
	}
	return false, ""
}

// GetPodContainerTerminateMessage returns the terminate message for a specific container of a pod
func GetPodContainerTerminateMessage(pod *corev1api.Pod, container string) string {
	message := ""
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == container {
			if containerStatus.State.Terminated != nil {
				message = containerStatus.State.Terminated.Message
			}
			break
		}
	}

	return message
}

// GetPodTerminateMessage returns the terminate message for all containers of a pod
func GetPodTerminateMessage(pod *corev1api.Pod) string {
	message := ""
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil {
			if containerStatus.State.Terminated.Message != "" {
				message += containerStatus.State.Terminated.Message + "/"
			}
		}
	}

	return message
}

func getPodLogReader(ctx context.Context, podGetter corev1client.CoreV1Interface, pod string, namespace string, logOptions *corev1api.PodLogOptions) (io.ReadCloser, error) {
	request := podGetter.Pods(namespace).GetLogs(pod, logOptions)
	return request.Stream(ctx)
}

var podLogReaderGetter = getPodLogReader

// CollectPodLogs collects logs of the specified container of a pod and write to the output
func CollectPodLogs(ctx context.Context, podGetter corev1client.CoreV1Interface, pod string, namespace string, container string, output io.Writer) error {
	logIndicator := fmt.Sprintf("***************************begin pod logs[%s/%s]***************************\n", pod, container)

	if _, err := output.Write([]byte(logIndicator)); err != nil {
		return errors.Wrap(err, "error to write begin pod log indicator")
	}

	logOptions := &corev1api.PodLogOptions{
		Container: container,
	}

	if input, err := podLogReaderGetter(ctx, podGetter, pod, namespace, logOptions); err != nil {
		logIndicator = fmt.Sprintf("No present log retrieved, err: %v\n", err)
	} else {
		if _, err := io.Copy(output, input); err != nil {
			return errors.Wrap(err, "error to copy input")
		}

		logIndicator = ""
	}

	logIndicator += fmt.Sprintf("***************************end pod logs[%s/%s]***************************\n", pod, container)
	if _, err := output.Write([]byte(logIndicator)); err != nil {
		return errors.Wrap(err, "error to write end pod log indicator")
	}

	return nil
}

func ToSystemAffinity(loadAffinities []*LoadAffinity) *corev1api.Affinity {
	if len(loadAffinities) == 0 {
		return nil
	}
	nodeSelectorTermList := make([]corev1api.NodeSelectorTerm, 0)

	for _, loadAffinity := range loadAffinities {
		requirements := []corev1api.NodeSelectorRequirement{}
		for k, v := range loadAffinity.NodeSelector.MatchLabels {
			requirements = append(requirements, corev1api.NodeSelectorRequirement{
				Key:      k,
				Values:   []string{v},
				Operator: corev1api.NodeSelectorOpIn,
			})
		}

		for _, exp := range loadAffinity.NodeSelector.MatchExpressions {
			requirements = append(requirements, corev1api.NodeSelectorRequirement{
				Key:      exp.Key,
				Values:   exp.Values,
				Operator: corev1api.NodeSelectorOperator(exp.Operator),
			})
		}

		nodeSelectorTermList = append(
			nodeSelectorTermList,
			corev1api.NodeSelectorTerm{
				MatchExpressions: requirements,
			},
		)
	}

	if len(nodeSelectorTermList) > 0 {
		result := new(corev1api.Affinity)
		result.NodeAffinity = new(corev1api.NodeAffinity)
		result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = new(corev1api.NodeSelector)
		result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = nodeSelectorTermList

		return result
	}

	return nil
}
