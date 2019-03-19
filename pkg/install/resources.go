/*
Copyright 2018, 2019 the Velero contributors.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
		ObjectMeta: objectMeta("", "velero"),
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
		ObjectMeta: objectMeta("", namespace),
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

func appendUnstructured(list *unstructured.UnstructuredList, obj runtime.Object) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		return err
	}
	list.Items = append(list.Items, unstructured.Unstructured{Object: u})
	return nil
}

// AllResources returns a slice of all resources necessary to install Velero into a Kubernetes cluster.
func AllResources(namespace, image, backupStorageProviderName, bucketName, prefix string) (*unstructured.UnstructuredList, error) {
	resources := new(unstructured.UnstructuredList)
	resources.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "List"})

	for _, crd := range CRDs() {
		unstructuredCRD, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&crd)
		// Remove the status field, as it's added with null values by the code, but is rejected by server-side validation
		delete(unstructuredCRD, "status")
		if err != nil {
			// TODO: Wrap the error
			return nil, err
		}
		resources.Items = append(resources.Items, unstructured.Unstructured{Object: unstructuredCRD})
	}

	ns := Namespace(namespace)
	appendUnstructured(resources, ns)

	crb := ClusterRoleBinding(namespace)
	appendUnstructured(resources, crb)

	sa := ServiceAccount(namespace)
	appendUnstructured(resources, sa)

	// TODO: pass config down.
	bsl := BackupStorageLocation(namespace, backupStorageProviderName, bucketName, prefix, nil)
	appendUnstructured(resources, bsl)

	vsl := VolumeSnapshotLocation(namespace, backupStorageProviderName, nil)
	appendUnstructured(resources, vsl)

	deploy := Deployment(namespace,
		WithImage(image),
	)
	appendUnstructured(resources, deploy)

	//TODO: Restic daemonset

	return resources, nil
}
