/*
Copyright The Velero Contributors.

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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type InternalLW struct {
	Client     kbclient.WithWatch
	Namespace  string
	ObjectList kbclient.ObjectList
}

func (lw *InternalLW) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lw.Client.Watch(context.Background(), lw.ObjectList, &kbclient.ListOptions{Raw: &options, Namespace: lw.Namespace})
}

func (lw *InternalLW) List(options metav1.ListOptions) (runtime.Object, error) {
	err := lw.Client.List(context.Background(), lw.ObjectList, &kbclient.ListOptions{Raw: &options, Namespace: lw.Namespace})
	return lw.ObjectList, err
}
