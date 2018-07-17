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

package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// DynamicFactory contains methods for retrieving dynamic clients for GroupVersionResources and
// GroupVersionKinds.
type DynamicFactory interface {
	// ClientForGroupVersionResource returns a Dynamic client for the given group/version
	// and resource for the given namespace.
	ClientForGroupVersionResource(gv schema.GroupVersion, resource metav1.APIResource, namespace string) (Dynamic, error)
}

// dynamicFactory implements DynamicFactory.
type dynamicFactory struct {
	dynamicClient dynamic.Interface
}

// NewDynamicFactory returns a new ClientPool-based dynamic factory.
func NewDynamicFactory(dynamicClient dynamic.Interface) DynamicFactory {
	return &dynamicFactory{dynamicClient: dynamicClient}
}

func (f *dynamicFactory) ClientForGroupVersionResource(gv schema.GroupVersion, resource metav1.APIResource, namespace string) (Dynamic, error) {
	return &dynamicResourceClient{
		resourceClient: f.dynamicClient.Resource(gv.WithResource(resource.Name)).Namespace(namespace),
	}, nil
}

// Creator creates an object.
type Creator interface {
	// Create creates an object.
	Create(obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// Lister lists objects.
type Lister interface {
	// List lists all the objects of a given resource.
	List(metav1.ListOptions) (runtime.Object, error)
}

// Watcher watches objects.
type Watcher interface {
	// Watch watches for changes to objects of a given resource.
	Watch(metav1.ListOptions) (watch.Interface, error)
}

// Getter gets an object.
type Getter interface {
	// Get fetches an object by name.
	Get(name string, opts metav1.GetOptions) (*unstructured.Unstructured, error)
}

// Patcher patches an object.
type Patcher interface {
	//Patch patches the named object using the provided patch bytes, which are expected to be in JSON merge patch format. The patched object is returned.

	Patch(name string, data []byte) (*unstructured.Unstructured, error)
}

// Dynamic contains client methods that Ark needs for backing up and restoring resources.
type Dynamic interface {
	Creator
	Lister
	Watcher
	Getter
	Patcher
}

// dynamicResourceClient implements Dynamic.
type dynamicResourceClient struct {
	resourceClient dynamic.ResourceInterface
}

var _ Dynamic = &dynamicResourceClient{}

func (d *dynamicResourceClient) Create(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return d.resourceClient.Create(obj)
}

func (d *dynamicResourceClient) List(options metav1.ListOptions) (runtime.Object, error) {
	return d.resourceClient.List(options)
}

func (d *dynamicResourceClient) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return d.resourceClient.Watch(options)
}

func (d *dynamicResourceClient) Get(name string, opts metav1.GetOptions) (*unstructured.Unstructured, error) {
	return d.resourceClient.Get(name, opts)
}

func (d *dynamicResourceClient) Patch(name string, data []byte) (*unstructured.Unstructured, error) {
	return d.resourceClient.Patch(name, types.MergePatchType, data)
}
