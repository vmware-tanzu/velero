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

// PVCBuilder builds PVC objects.
type PVCBuilder struct {
	*corev1.PersistentVolumeClaim
}

func (p *PVCBuilder) Result() *corev1.PersistentVolumeClaim {
	return p.PersistentVolumeClaim
}

func NewPVC(ns, name string) *PVCBuilder {
	oMeta := metav1.ObjectMeta{Name: name, Namespace: ns}
	return &PVCBuilder{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: oMeta,
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce, // Default read write once
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"), // Default 1Gi
					},
				},
			},
		},
	}
}

func (p *PVCBuilder) WithAnnotation(ann map[string]string) *PVCBuilder {
	p.Annotations = ann
	return p
}

func (p *PVCBuilder) WithStorageClass(sc string) *PVCBuilder {
	p.Spec.StorageClassName = &sc
	return p
}

func (p *PVCBuilder) WithResourceStorage(q resource.Quantity) *PVCBuilder {
	p.Spec.Resources.Requests[corev1.ResourceStorage] = q
	return p
}

func CreatePVC(client TestClient, ns, name, sc string, ann map[string]string) (*corev1.PersistentVolumeClaim, error) {
	pvcBulder := NewPVC(ns, name)
	if ann != nil {
		pvcBulder.WithAnnotation(ann)
	}
	if sc != "" {
		pvcBulder.WithStorageClass(sc)
	}

	return client.ClientGo.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvcBulder.Result(), metav1.CreateOptions{})
}

func CreatePvc(client TestClient, pvcBulder *PVCBuilder) error {
	_, err := client.ClientGo.CoreV1().PersistentVolumeClaims(pvcBulder.Namespace).Create(context.TODO(), pvcBulder.Result(), metav1.CreateOptions{})
	return err
}

func GetPVC(ctx context.Context, client TestClient, namespace string, pvcName string) (*corev1.PersistentVolumeClaim, error) {
	return client.ClientGo.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
}
