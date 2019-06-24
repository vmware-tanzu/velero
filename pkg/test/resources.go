/*
Copyright 2019 the Velero contributors.

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

package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// APIResource stores information about a specific Kubernetes API
// resource.
type APIResource struct {
	Group      string
	Version    string
	Name       string
	ShortName  string
	Namespaced bool
	Items      []metav1.Object
}

// GVR returns a GroupVersionResource representing the resource.
func (r *APIResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Name,
	}
}

// Pods returns an APIResource describing core/v1's Pods.
func Pods(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "pods",
		ShortName:  "po",
		Namespaced: true,
		Items:      items,
	}
}

func PVCs(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "persistentvolumeclaims",
		ShortName:  "pvc",
		Namespaced: true,
		Items:      items,
	}
}

func PVs(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "persistentvolumes",
		ShortName:  "pv",
		Namespaced: false,
		Items:      items,
	}
}

func Secrets(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "secrets",
		ShortName:  "secrets",
		Namespaced: true,
		Items:      items,
	}
}

func Deployments(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "apps",
		Version:    "v1",
		Name:       "deployments",
		ShortName:  "deploy",
		Namespaced: true,
		Items:      items,
	}
}

func ExtensionsDeployments(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "extensions",
		Version:    "v1",
		Name:       "deployments",
		ShortName:  "deploy",
		Namespaced: true,
		Items:      items,
	}
}

func Namespaces(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "namespaces",
		ShortName:  "ns",
		Namespaced: false,
		Items:      items,
	}
}

func NewPod(ns, name string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}
}

func NewPVC(ns, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}
}

func NewPV(name string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta("", name),
	}
}

func NewSecret(ns, name string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}
}

func NewDeployment(ns, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}
}

func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta("", name),
	}
}

func objectMeta(ns, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: ns,
		Name:      name,
	}
}
