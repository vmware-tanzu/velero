package k8s

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetContainers(podItem unstructured.Unstructured) ([]Container, error) {
	var containers []Container
	coreContainers, err := _getPodContainers(podItem)
	if err != nil {
		return containers, err
	}

	for _, c := range coreContainers {
		containers = append(containers, NewContainerLogger(podItem.GetNamespace(), podItem.GetName(), c))
	}
	return containers, nil
}

func _getPodContainers(podItem unstructured.Unstructured) ([]corev1.Container, error) {
	var containers []corev1.Container

	pod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(podItem.Object, &pod); err != nil {
		return nil, fmt.Errorf("error converting container objects: %s", err)
	}

	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	containers = append(containers, _getPodEphemeralContainers(pod)...)
	return containers, nil
}

func _getPodEphemeralContainers(pod *corev1.Pod) []corev1.Container {
	var containers []corev1.Container
	for _, ec := range pod.Spec.EphemeralContainers {
		containers = append(containers, corev1.Container(ec.EphemeralContainerCommon))
	}
	return containers
}
