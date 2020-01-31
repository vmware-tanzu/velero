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
	appsv1api "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentBuilder builds Deployment objects.
type DeploymentBuilder struct {
	object *appsv1api.Deployment
}

// ForDeployment is the constructor for a DeploymentBuilder.
func ForDeployment(ns, name string) *DeploymentBuilder {
	return &DeploymentBuilder{
		object: &appsv1api.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1api.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

// Result returns the built Deployment.
func (b *DeploymentBuilder) Result() *appsv1api.Deployment {
	return b.object
}

// ObjectMeta applies functional options to the Deployment's ObjectMeta.
func (b *DeploymentBuilder) ObjectMeta(opts ...ObjectMetaOpt) *DeploymentBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}
