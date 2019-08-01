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

package test

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type FakeNamespaceClient struct {
	mock.Mock
}

var _ corev1.NamespaceInterface = &FakeNamespaceClient{}

func (c *FakeNamespaceClient) List(options metav1.ListOptions) (*v1.NamespaceList, error) {
	args := c.Called(options)
	return args.Get(0).(*v1.NamespaceList), args.Error(1)
}

func (c *FakeNamespaceClient) Create(obj *v1.Namespace) (*v1.Namespace, error) {
	args := c.Called(obj)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Watch(options metav1.ListOptions) (watch.Interface, error) {
	args := c.Called(options)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (c *FakeNamespaceClient) Get(name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	args := c.Called(name, opts)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*v1.Namespace, error) {
	args := c.Called(name, pt, data, subresources)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Delete(name string, opts *metav1.DeleteOptions) error {
	args := c.Called(name, opts)
	return args.Error(1)
}

func (c *FakeNamespaceClient) Finalize(item *v1.Namespace) (*v1.Namespace, error) {
	args := c.Called(item)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Update(namespace *v1.Namespace) (*v1.Namespace, error) {
	args := c.Called(namespace)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) UpdateStatus(namespace *v1.Namespace) (*v1.Namespace, error) {
	args := c.Called(namespace)
	return args.Get(0).(*v1.Namespace), args.Error(1)
}
