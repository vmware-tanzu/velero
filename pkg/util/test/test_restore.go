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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

type TestRestore struct {
	*api.Restore
}

func NewTestRestore(ns, name string, phase api.RestorePhase) *TestRestore {
	return &TestRestore{
		Restore: &api.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Spec: api.RestoreSpec{},
			Status: api.RestoreStatus{
				Phase: phase,
			},
		},
	}
}

func (r *TestRestore) WithRestorableNamespace(name string) *TestRestore {
	r.Spec.Namespaces = append(r.Spec.Namespaces, name)
	return r
}

func (r *TestRestore) WithValidationError(err string) *TestRestore {
	r.Status.ValidationErrors = append(r.Status.ValidationErrors, err)
	return r
}

func (r *TestRestore) WithBackup(name string) *TestRestore {
	r.Spec.BackupName = name
	return r
}

func (r *TestRestore) WithErrors(e api.RestoreResult) *TestRestore {
	r.Status.Errors = e
	return r
}

func (r *TestRestore) WithRestorePVs(value bool) *TestRestore {
	r.Spec.RestorePVs = value
	return r
}
