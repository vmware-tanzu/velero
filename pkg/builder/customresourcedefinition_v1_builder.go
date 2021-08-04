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
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinitionV1Builder builds v1 CustomResourceDefinition objects.
type CustomResourceDefinitionV1Builder struct {
	object *apiextv1.CustomResourceDefinition
}

// ForCustomResourceDefinitionV1 is the constructor for a CustomResourceDefinitionV1Builder.
func ForCustomResourceDefinitionV1(name string) *CustomResourceDefinitionV1Builder {
	return &CustomResourceDefinitionV1Builder{
		object: &apiextv1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiextv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

// Condition adds a CustomResourceDefinitionCondition objects to a CustomResourceDefinitionV1Builder.
func (c *CustomResourceDefinitionV1Builder) Condition(cond apiextv1.CustomResourceDefinitionCondition) *CustomResourceDefinitionV1Builder {
	c.object.Status.Conditions = append(c.object.Status.Conditions, cond)
	return c
}

// Result returns the built V1 CustomResourceDefinition.
func (b *CustomResourceDefinitionV1Builder) Result() *apiextv1.CustomResourceDefinition {
	return b.object
}

// ObjectMeta applies functional options to the CustomResourceDefinition's ObjectMeta.
func (b *CustomResourceDefinitionV1Builder) ObjectMeta(opts ...ObjectMetaOpt) *CustomResourceDefinitionV1Builder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// CustomResourceDefinitionV1ConditionBuilder builds CustomResourceDefinitionV1Condition objects.
type CustomResourceDefinitionV1ConditionBuilder struct {
	object apiextv1.CustomResourceDefinitionCondition
}

// ForCustomResourceDefinitionV1Condition is the construction for a CustomResourceDefinitionV1ConditionBuilder.
func ForCustomResourceDefinitionV1Condition() *CustomResourceDefinitionV1ConditionBuilder {
	return &CustomResourceDefinitionV1ConditionBuilder{
		object: apiextv1.CustomResourceDefinitionCondition{},
	}
}

// Type sets the Condition's type.
func (c *CustomResourceDefinitionV1ConditionBuilder) Type(t apiextv1.CustomResourceDefinitionConditionType) *CustomResourceDefinitionV1ConditionBuilder {
	c.object.Type = t
	return c
}

// Status sets the Condition's status.
func (c *CustomResourceDefinitionV1ConditionBuilder) Status(cs apiextv1.ConditionStatus) *CustomResourceDefinitionV1ConditionBuilder {
	c.object.Status = cs
	return c
}

// Result returns the built v1 CustomResourceDefinitionCondition.
func (c *CustomResourceDefinitionV1ConditionBuilder) Result() apiextv1.CustomResourceDefinitionCondition {
	return c.object
}
