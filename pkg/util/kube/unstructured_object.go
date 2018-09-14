package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// UnstructuredObject is a Kubernetes object stored as a
// map[string]interface{}, with object metadata accessors.
type UnstructuredObject interface {
	runtime.Unstructured
	metav1.Object
}
