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

package k8s

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePod(client TestClient, ns, name string, volumes []corev1.Volume) (*corev1.Pod, error) {
	vmList := []corev1.VolumeMount{}
	for _, v := range volumes {
		vmList = append(vmList, corev1.VolumeMount{
			Name:      v.Name,
			MountPath: "/" + v.Name,
		})
	}
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:         name,
					Image:        "gcr.io/velero-gcp/busybox",
					Command:      []string{"sleep", "3600"},
					VolumeMounts: vmList,
				},
			},
			Volumes: volumes,
		},
	}

	return client.ClientGo.CoreV1().Pods(ns).Create(context.TODO(), p, metav1.CreateOptions{})
}

func GetPod(ctx context.Context, client TestClient, namespace string, pod string) (*corev1.Pod, error) {
	return client.ClientGo.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
}

func AddAnnotationToPod(ctx context.Context, client TestClient, namespace, podName string, ann map[string]string) (*corev1.Pod, error) {

	newPod, err := GetPod(ctx, client, namespace, podName)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Fail to ge pod %s in namespace %s", podName, namespace))
	}
	newAnn := newPod.ObjectMeta.Annotations
	if newAnn == nil {
		newAnn = make(map[string]string)
	}
	for k, v := range ann {
		fmt.Println(k, v)
		newAnn[k] = v
	}
	newPod.Annotations = newAnn
	fmt.Println(newPod.Annotations)

	return client.ClientGo.CoreV1().Pods(namespace).Update(ctx, newPod, metav1.UpdateOptions{})
}

func GetPodNodeName(ctx context.Context, client TestClient, namespace, podName string) (string, error) {
	pod, err := GetPod(ctx, client, namespace, podName)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Fail to ge pod %s in namespace %s", podName, namespace))
	}
	return pod.Spec.NodeName, nil
}
