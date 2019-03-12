/*
Copyright 2018 the Velero contributors.

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

package install

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heptio/velero/pkg/apis/velero/v1"
)

func labels() map[string]string {
	return map[string]string{
		"component": "velero",
	}
}

func podAnnotations() map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "8085",
		"prometheus.io/path":   "/metrics",
	}
}

func containerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "metrics",
			ContainerPort: 8085,
		},
	}
}

func objectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels(),
	}
}

func ServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: objectMeta(namespace, "velero"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
}

func ClusterRoleBinding(namespace string) *rbacv1beta1.ClusterRoleBinding {
	crb := &rbacv1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "velero",
			Labels: labels(),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1beta1.SchemeGroupVersion.String(),
		},
		Subjects: []rbacv1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      "velero",
			},
		},
		RoleRef: rbacv1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return crb
}

func Namespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: labels(),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
}

func BackupStorageLocation(namespace, provider, bucket, prefix string, config map[string]string) *v1.BackupStorageLocation {
	return &v1.BackupStorageLocation{
		ObjectMeta: objectMeta(namespace, "default"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "BackupStorageLocation",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		Spec: v1.BackupStorageLocationSpec{
			Provider: provider,
			StorageType: v1.StorageType{
				ObjectStorage: &v1.ObjectStorageLocation{
					Bucket: bucket,
					Prefix: prefix,
				},
			},
			Config: config,
		},
	}
}

func VolumeSnapshotLocation(namespace, provider string, config map[string]string) *v1.VolumeSnapshotLocation {
	return &v1.VolumeSnapshotLocation{
		ObjectMeta: objectMeta(namespace, "default"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumeSnapshotLocation",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		Spec: v1.VolumeSnapshotLocationSpec{
			Provider: provider,
			Config:   config,
		},
	}
}
