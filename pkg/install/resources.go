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
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/generated/crds"
)

// Use "latest" if the build process didn't supply a version
func imageVersion() string {
	if buildinfo.Version == "" {
		return "latest"
	}
	return buildinfo.Version
}

// DefaultImage is the default image to use for the Velero deployment and restic daemonset containers.
var (
	DefaultImage               = "velero/velero:" + imageVersion()
	DefaultVeleroPodCPURequest = "500m"
	DefaultVeleroPodMemRequest = "128Mi"
	DefaultVeleroPodCPULimit   = "1000m"
	DefaultVeleroPodMemLimit   = "256Mi"
	DefaultResticPodCPURequest = "0"
	DefaultResticPodMemRequest = "0"
	DefaultResticPodCPULimit   = "0"
	DefaultResticPodMemLimit   = "0"
)

func labels() map[string]string {
	return map[string]string{
		"component": "velero",
	}
}

func podAnnotations(userAnnotations map[string]string) map[string]string {
	// Use the default annotations as a starting point
	base := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "8085",
		"prometheus.io/path":   "/metrics",
	}

	// Merge base annotations with user annotations to enforce CLI precedence
	for k, v := range userAnnotations {
		base[k] = v
	}

	return base
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

func ServiceAccount(namespace string, annotations map[string]string) *corev1.ServiceAccount {
	objMeta := objectMeta(namespace, "velero")
	objMeta.Annotations = annotations
	return &corev1.ServiceAccount{
		ObjectMeta: objMeta,
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

func BackupStorageLocation(namespace, provider, bucket, prefix string, config map[string]string, caCert []byte) *v1.BackupStorageLocation {
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
					CACert: caCert,
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

func Secret(namespace string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: objectMeta(namespace, "cloud-credentials"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Data: map[string][]byte{
			"cloud": data,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func appendUnstructured(list *unstructured.UnstructuredList, obj runtime.Object) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)

	// Remove the status field so we're not sending blank data to the server.
	// On CRDs, having an empty status is actually a validation error.
	delete(u, "status")
	if err != nil {
		return err
	}
	list.Items = append(list.Items, unstructured.Unstructured{Object: u})
	return nil
}

type VeleroOptions struct {
	Namespace                         string
	Image                             string
	ProviderName                      string
	Bucket                            string
	Prefix                            string
	PodAnnotations                    map[string]string
	ServiceAccountAnnotations         map[string]string
	VeleroPodResources                corev1.ResourceRequirements
	ResticPodResources                corev1.ResourceRequirements
	SecretData                        []byte
	RestoreOnly                       bool
	UseRestic                         bool
	UseVolumeSnapshots                bool
	BSLConfig                         map[string]string
	VSLConfig                         map[string]string
	DefaultResticMaintenanceFrequency time.Duration
	Plugins                           []string
	NoDefaultBackupLocation           bool
	CACertData                        []byte
}

func AllCRDs() *unstructured.UnstructuredList {
	resources := new(unstructured.UnstructuredList)
	// Set the GVK so that the serialization framework outputs the list properly
	resources.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "List"})

	for _, crd := range crds.CRDs {
		crd.SetLabels(labels())
		appendUnstructured(resources, crd)
	}

	return resources
}

// AllResources returns a list of all resources necessary to install Velero, in the appropriate order, into a Kubernetes cluster.
// Items are unstructured, since there are different data types returned.
func AllResources(o *VeleroOptions) (*unstructured.UnstructuredList, error) {
	resources := AllCRDs()

	ns := Namespace(o.Namespace)
	appendUnstructured(resources, ns)

	crb := ClusterRoleBinding(o.Namespace)
	appendUnstructured(resources, crb)

	sa := ServiceAccount(o.Namespace, o.ServiceAccountAnnotations)
	appendUnstructured(resources, sa)

	if o.SecretData != nil {
		sec := Secret(o.Namespace, o.SecretData)
		appendUnstructured(resources, sec)
	}

	if !o.NoDefaultBackupLocation {
		bsl := BackupStorageLocation(o.Namespace, o.ProviderName, o.Bucket, o.Prefix, o.BSLConfig, o.CACertData)
		appendUnstructured(resources, bsl)
	}

	// A snapshot location may not be desirable for users relying on restic
	if o.UseVolumeSnapshots {
		vsl := VolumeSnapshotLocation(o.Namespace, o.ProviderName, o.VSLConfig)
		appendUnstructured(resources, vsl)
	}

	secretPresent := o.SecretData != nil

	deployOpts := []podTemplateOption{
		WithAnnotations(o.PodAnnotations),
		WithImage(o.Image),
		WithResources(o.VeleroPodResources),
		WithSecret(secretPresent),
		WithDefaultResticMaintenanceFrequency(o.DefaultResticMaintenanceFrequency),
	}

	if o.RestoreOnly {
		deployOpts = append(deployOpts, WithRestoreOnly())
	}

	if len(o.Plugins) > 0 {
		deployOpts = append(deployOpts, WithPlugins(o.Plugins))
	}

	deploy := Deployment(o.Namespace, deployOpts...)

	appendUnstructured(resources, deploy)

	if o.UseRestic {
		ds := DaemonSet(o.Namespace,
			WithAnnotations(o.PodAnnotations),
			WithImage(o.Image),
			WithResources(o.ResticPodResources),
			WithSecret(secretPresent),
		)
		appendUnstructured(resources, ds)
	}

	return resources, nil
}
