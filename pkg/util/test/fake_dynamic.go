/*
Copyright 2017 Heptio Inc.

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
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/heptio/ark/pkg/client"
)

type FakeDynamicFactory struct {
	mock.Mock
}

var _ client.DynamicFactory = &FakeDynamicFactory{}

func (df *FakeDynamicFactory) ClientForGroupVersionResource(gv schema.GroupVersion, resource metav1.APIResource, namespace string) (client.Dynamic, error) {
	args := df.Called(gv, resource, namespace)
	return args.Get(0).(client.Dynamic), args.Error(1)
}

type FakeDynamicClient struct {
	mock.Mock
}

var _ client.Dynamic = &FakeDynamicClient{}

func (c *FakeDynamicClient) List(options metav1.ListOptions) (runtime.Object, error) {
	args := c.Called(options)
	return args.Get(0).(runtime.Object), args.Error(1)
}

func (c *FakeDynamicClient) Create(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	args := c.Called(obj)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (c *FakeDynamicClient) Watch(options metav1.ListOptions) (watch.Interface, error) {
	args := c.Called(options)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (c *FakeDynamicClient) Get(name string, opts metav1.GetOptions) (*unstructured.Unstructured, error) {
	args := c.Called(name, opts)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}
