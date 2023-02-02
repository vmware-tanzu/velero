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

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolume(client TestClient, name string) (*corev1.PersistentVolume, error) {

	p := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "manual",
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},

			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/demo",
				},
			},
		},
	}

	return client.ClientGo.CoreV1().PersistentVolumes().Create(context.TODO(), p, metav1.CreateOptions{})
}

func GetPersistentVolume(ctx context.Context, client TestClient, namespace string, persistentVolume string) (*corev1.PersistentVolume, error) {
	return client.ClientGo.CoreV1().PersistentVolumes().Get(ctx, persistentVolume, metav1.GetOptions{})
}

func AddAnnotationToPersistentVolume(ctx context.Context, client TestClient, namespace string, persistentVolume, key string) (*corev1.PersistentVolume, error) {
	newPV, err := GetPersistentVolume(ctx, client, "", persistentVolume)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Fail to ge PV %s", persistentVolume))
	}
	ann := newPV.ObjectMeta.Annotations
	ann[key] = persistentVolume
	newPV.Annotations = ann

	return client.ClientGo.CoreV1().PersistentVolumes().Update(ctx, newPV, metav1.UpdateOptions{})
}
