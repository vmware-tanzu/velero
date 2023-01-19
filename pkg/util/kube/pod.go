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
	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
)

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
