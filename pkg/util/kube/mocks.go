package kube

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client knows how to perform CRUD operations on Kubernetes objects.
//
//go:generate mockery --name=Client
type Client interface {
	client.Reader
	client.Writer
	client.StatusClient
	client.SubResourceClientConstructor

	// Scheme returns the scheme this client is using.
	Scheme() *runtime.Scheme
	// RESTMapper returns the rest this client is using.
	RESTMapper() meta.RESTMapper
	// GroupVersionKindFor returns the GroupVersionKind for the given object.
	GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error)
	// IsObjectNamespaced returns true if the GroupVersionKind of the object is namespaced.
	IsObjectNamespaced(obj runtime.Object) (bool, error)
}
