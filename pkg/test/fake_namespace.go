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
	"context"

	"github.com/stretchr/testify/mock"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type FakeNamespaceClient struct {
	mock.Mock
}

var _ corev1.NamespaceInterface = &FakeNamespaceClient{}

func (c *FakeNamespaceClient) List(ctx context.Context, options metav1.ListOptions) (*corev1api.NamespaceList, error) {
	args := c.Called(options)
	return args.Get(0).(*corev1api.NamespaceList), args.Error(1)
}

func (c *FakeNamespaceClient) Create(ctx context.Context, obj *corev1api.Namespace, options metav1.CreateOptions) (*corev1api.Namespace, error) {
	args := c.Called(obj)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Watch(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
	args := c.Called(options)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (c *FakeNamespaceClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1api.Namespace, error) {
	args := c.Called(name, opts)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*corev1api.Namespace, error) {
	args := c.Called(name, pt, data, subresources)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	args := c.Called(name, opts)
	return args.Error(1)
}

func (c *FakeNamespaceClient) Finalize(ctx context.Context, item *corev1api.Namespace, options metav1.UpdateOptions) (*corev1api.Namespace, error) {
	args := c.Called(item)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) Update(ctx context.Context, namespace *corev1api.Namespace, options metav1.UpdateOptions) (*corev1api.Namespace, error) {
	args := c.Called(namespace)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}

func (c *FakeNamespaceClient) UpdateStatus(ctx context.Context, namespace *corev1api.Namespace, options metav1.UpdateOptions) (*corev1api.Namespace, error) {
	args := c.Called(namespace)
	return args.Get(0).(*corev1api.Namespace), args.Error(1)
}
