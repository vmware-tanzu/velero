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

package builder

import (
	batchv1api "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobBuilder struct {
	object *batchv1api.Job
}

// ForJob is the constructor for a JobBuilder.
func ForJob(ns, name string) *JobBuilder {
	return &JobBuilder{
		object: &batchv1api.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: batchv1api.SchemeGroupVersion.String(),
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built Job.
func (j *JobBuilder) Result() *batchv1api.Job {
	return j.object
}

// ObjectMeta applies functional options to the Job's ObjectMeta.
func (j *JobBuilder) ObjectMeta(opts ...ObjectMetaOpt) *JobBuilder {
	for _, opt := range opts {
		opt(j.object)
	}

	return j
}

// Succeeded sets Succeeded on the Job's Status.
func (j *JobBuilder) Succeeded(succeeded int) *JobBuilder {
	j.object.Status.Succeeded = int32(succeeded)
	return j
}
