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

package builder

import (
	storagev1api "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClassBuilder builds StorageClass objects.
type StorageClassBuilder struct {
	object *storagev1api.StorageClass
}

// ForStorageClass is the constructor for a StorageClassBuilder.
func ForStorageClass(name string) *StorageClassBuilder {
	return &StorageClassBuilder{
		object: &storagev1api.StorageClass{
			TypeMeta: metav1.TypeMeta{
				APIVersion: storagev1api.SchemeGroupVersion.String(),
				Kind:       "StorageClass",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Result returns the built StorageClass.
func (b *StorageClassBuilder) Result() *storagev1api.StorageClass {
	return b.object
}

// ObjectMeta applies functional options to the StorageClass's ObjectMeta.
func (b *StorageClassBuilder) ObjectMeta(opts ...ObjectMetaOpt) *StorageClassBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
