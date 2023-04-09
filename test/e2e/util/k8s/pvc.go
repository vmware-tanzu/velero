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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePVC(client TestClient, ns, name, sc string, ann map[string]string) (*corev1.PersistentVolumeClaim, error) {
	oMeta := metav1.ObjectMeta{}
	oMeta = metav1.ObjectMeta{Name: name}
	if ann != nil {
		oMeta.Annotations = ann
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: oMeta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Mi"),
				},
			},
			StorageClassName: &sc,
		},
	}
	return client.ClientGo.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvc, metav1.CreateOptions{})
}
func GetPVC(ctx context.Context, client TestClient, namespace string, pvcName string) (*corev1.PersistentVolumeClaim, error) {
	return client.ClientGo.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
}
