/*
Copyright the Velero contributors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// BackupRepositoryBuilder builds BackupRepository objects.
type BackupRepositoryBuilder struct {
	object *velerov1api.BackupRepository
}

// ForBackupRepository is the constructor for a BackupRepositoryBuilder.
func ForBackupRepository(ns, name string) *BackupRepositoryBuilder {
	return &BackupRepositoryBuilder{
		object: &velerov1api.BackupRepository{
			Spec: velerov1api.BackupRepositorySpec{ResticIdentifier: ""},
			TypeMeta: metav1.TypeMeta{
				APIVersion: velerov1api.SchemeGroupVersion.String(),
				Kind:       "BackupRepository",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built BackupRepository.
func (b *BackupRepositoryBuilder) Result() *velerov1api.BackupRepository {
	return b.object
}

// ObjectMeta applies functional options to the BackupRepository's ObjectMeta.
func (b *BackupRepositoryBuilder) ObjectMeta(opts ...ObjectMetaOpt) *BackupRepositoryBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
