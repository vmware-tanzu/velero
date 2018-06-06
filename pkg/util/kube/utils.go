/*
Copyright 2017 the Heptio Ark contributors.

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
	"fmt"

	"github.com/pkg/errors"

	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// NamespaceAndName returns a string in the format <namespace>/<name>
func NamespaceAndName(objMeta metav1.Object) string {
	if objMeta.GetNamespace() == "" {
		return objMeta.GetName()
	}
	return fmt.Sprintf("%s/%s", objMeta.GetNamespace(), objMeta.GetName())
}

// EnsureNamespaceExists attempts to create the provided Kubernetes namespace. It returns two values:
// a bool indicating whether or not the namespace was created, and an error if the create failed
// for a reason other than that the namespace already exists. Note that in the case where the
// namespace already exists, this function will return (false, nil).
func EnsureNamespaceExists(namespace *corev1api.Namespace, client corev1client.NamespaceInterface) (bool, error) {
	if _, err := client.Create(namespace); err == nil {
		return true, nil
	} else if apierrors.IsAlreadyExists(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "error creating namespace %s", namespace.Name)
	}
}

// GetVolumeDirectory gets the name of the directory on the host, under /var/lib/kubelet/pods/<podUID>/volumes/,
// where the specified volume lives.
func GetVolumeDirectory(pod *corev1api.Pod, volumeName string, pvcLister corev1listers.PersistentVolumeClaimLister) (string, error) {
	var volume *corev1api.Volume

	for _, item := range pod.Spec.Volumes {
		if item.Name == volumeName {
			volume = &item
			break
		}
	}

	if volume == nil {
		return "", errors.New("volume not found in pod")
	}

	if volume.VolumeSource.PersistentVolumeClaim == nil {
		return volume.Name, nil
	}

	pvc, err := pvcLister.PersistentVolumeClaims(pod.Namespace).Get(volume.VolumeSource.PersistentVolumeClaim.ClaimName)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return pvc.Spec.VolumeName, nil
}
