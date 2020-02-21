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
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinitionBuilder builds CustomResourceDefinition objects.
type CustomResourceDefinitionBuilder struct {
	object *apiextv1beta1.CustomResourceDefinition
}

// ForCustomResourceDefinition is the constructor for a CustomResourceDefinitionBuilder.
func ForCustomResourceDefinition(name string) *CustomResourceDefinitionBuilder {
	return &CustomResourceDefinitionBuilder{
		object: &apiextv1beta1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiextv1beta1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Condition adds a CustomResourceDefinitionCondition objects to a CustomResourceDefinitionBuilder.
func (c *CustomResourceDefinitionBuilder) Condition(cond apiextv1beta1.CustomResourceDefinitionCondition) *CustomResourceDefinitionBuilder {
	c.object.Status.Conditions = append(c.object.Status.Conditions, cond)
	return c
}

// Result returns the built CustomResourceDefinition.
func (b *CustomResourceDefinitionBuilder) Result() *apiextv1beta1.CustomResourceDefinition {
	return b.object
}

// ObjectMeta applies functional options to the CustomResourceDefinition's ObjectMeta.
func (b *CustomResourceDefinitionBuilder) ObjectMeta(opts ...ObjectMetaOpt) *CustomResourceDefinitionBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// CustomResourceDefinitionConditionBuilder builds CustomResourceDefinitionCondition objects.
type CustomResourceDefinitionConditionBuilder struct {
	object apiextv1beta1.CustomResourceDefinitionCondition
}

// ForCustomResourceDefinitionConditionBuilder is the construction for a CustomResourceDefinitionConditionBuilder.
func ForCustomResourceDefinitionCondition() *CustomResourceDefinitionConditionBuilder {
	return &CustomResourceDefinitionConditionBuilder{
		object: apiextv1beta1.CustomResourceDefinitionCondition{},
	}
}

// Type sets the Condition's type.
func (c *CustomResourceDefinitionConditionBuilder) Type(t apiextv1beta1.CustomResourceDefinitionConditionType) *CustomResourceDefinitionConditionBuilder {
	c.object.Type = t
	return c
}

// Status sets the Condition's status.
func (c *CustomResourceDefinitionConditionBuilder) Status(cs apiextv1beta1.ConditionStatus) *CustomResourceDefinitionConditionBuilder {
	c.object.Status = cs
	return c
}

// Results returns the built CustomResourceDefinitionCondition.
func (b *CustomResourceDefinitionConditionBuilder) Result() apiextv1beta1.CustomResourceDefinitionCondition {
	return b.object
}
