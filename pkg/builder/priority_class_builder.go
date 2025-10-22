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
	corev1api "k8s.io/api/core/v1"
	schedulingv1api "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PriorityClassBuilder struct {
	object *schedulingv1api.PriorityClass
}

func ForPriorityClass(name string) *PriorityClassBuilder {
	return &PriorityClassBuilder{
		object: &schedulingv1api.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

func (p *PriorityClassBuilder) Value(value int) *PriorityClassBuilder {
	p.object.Value = int32(value)
	return p
}

func (p *PriorityClassBuilder) PreemptionPolicy(policy string) *PriorityClassBuilder {
	preemptionPolicy := corev1api.PreemptionPolicy(policy)
	p.object.PreemptionPolicy = &preemptionPolicy
	return p
}

func (p *PriorityClassBuilder) Result() *schedulingv1api.PriorityClass {
	return p.object
}
