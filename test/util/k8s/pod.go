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
	"path"

	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

func CreatePod(
	client TestClient,
	ns, name, sc, pvcName string,
	volumeNameList []string,
	pvcAnn, ann map[string]string,
	imageRegistryProxy string,
) (*corev1api.Pod, error) {
	if pvcName != "" && len(volumeNameList) != 1 {
		return nil, errors.New("Volume name list should contain only 1 since PVC name is not empty")
	}

	imageAddress := TestImage
	if imageRegistryProxy != "" {
		imageAddress = path.Join(imageRegistryProxy, TestImage)
	}

	volumes := []corev1api.Volume{}
	for _, volume := range volumeNameList {
		var _pvcName string
		if pvcName == "" {
			_pvcName = fmt.Sprintf("pvc-%s", volume)
		} else {
			_pvcName = pvcName
		}
		pvc, err := CreatePVC(client, ns, _pvcName, sc, pvcAnn)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, corev1api.Volume{
			Name: volume,
			VolumeSource: corev1api.VolumeSource{
				PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		})
	}

	vmList := []corev1api.VolumeMount{}
	for _, v := range volumes {
		vmList = append(vmList, corev1api.VolumeMount{
			Name:      v.Name,
			MountPath: "/" + v.Name,
		})
	}

	p := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: ann,
		},
		Spec: corev1api.PodSpec{
			SecurityContext: &corev1api.PodSecurityContext{
				FSGroup:             func(i int64) *int64 { return &i }(65534),
				FSGroupChangePolicy: func(policy corev1api.PodFSGroupChangePolicy) *corev1api.PodFSGroupChangePolicy { return &policy }(corev1api.FSGroupChangeAlways),
			},
			Containers: []corev1api.Container{
				{
					Name:         name,
					Image:        imageAddress,
					Command:      []string{"sleep", "3600"},
					VolumeMounts: vmList,
					// Make pod obeys the restricted pod security standards.
					SecurityContext: &corev1api.SecurityContext{
						AllowPrivilegeEscalation: boolptr.False(),
						Capabilities: &corev1api.Capabilities{
							Drop: []corev1api.Capability{"ALL"},
						},
						RunAsNonRoot: boolptr.True(),
						RunAsUser:    func(i int64) *int64 { return &i }(65534),
						RunAsGroup:   func(i int64) *int64 { return &i }(65534),
						SeccompProfile: &corev1api.SeccompProfile{
							Type: corev1api.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			Volumes: volumes,
		},
	}

	return client.ClientGo.CoreV1().Pods(ns).Create(context.TODO(), p, metav1.CreateOptions{})
}

func GetPod(ctx context.Context, client TestClient, namespace string, pod string) (*corev1api.Pod, error) {
	return client.ClientGo.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
}

func AddAnnotationToPod(ctx context.Context, client TestClient, namespace, podName string, ann map[string]string) (*corev1api.Pod, error) {
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

func ListPods(ctx context.Context, client TestClient, namespace string) (*corev1api.PodList, error) {
	return client.ClientGo.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}
