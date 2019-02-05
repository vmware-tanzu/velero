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

package test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
)

type TestBackupStorageLocation struct {
	*v1.BackupStorageLocation
}

func NewTestBackupStorageLocation() *TestBackupStorageLocation {
	return &TestBackupStorageLocation{
		BackupStorageLocation: &v1.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1.DefaultNamespace,
			},
		},
	}
}

func (b *TestBackupStorageLocation) WithNamespace(namespace string) *TestBackupStorageLocation {
	b.Namespace = namespace
	return b
}

func (b *TestBackupStorageLocation) WithName(name string) *TestBackupStorageLocation {
	b.Name = name
	return b
}

func (b *TestBackupStorageLocation) WithLabel(key, value string) *TestBackupStorageLocation {
	if b.Labels == nil {
		b.Labels = make(map[string]string)
	}
	b.Labels[key] = value
	return b
}

func (b *TestBackupStorageLocation) WithProvider(name string) *TestBackupStorageLocation {
	b.Spec.Provider = name
	return b
}

func (b *TestBackupStorageLocation) WithObjectStorage(bucketName string) *TestBackupStorageLocation {
	if b.Spec.StorageType.ObjectStorage == nil {
		b.Spec.StorageType.ObjectStorage = &v1.ObjectStorageLocation{}
	}
	b.Spec.ObjectStorage.Bucket = bucketName
	return b
}
