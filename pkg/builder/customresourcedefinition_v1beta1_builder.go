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
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinitionV1Beta1Builder builds v1beta1 CustomResourceDefinition objects.
type CustomResourceDefinitionV1Beta1Builder struct {
	object *apiextv1beta1.CustomResourceDefinition
}

// ForCustomResourceDefinitionV1Beta1 is the constructor for a CustomResourceDefinitionV1Beta1Builder.
func ForCustomResourceDefinitionV1Beta1(name string) *CustomResourceDefinitionV1Beta1Builder {
	return &CustomResourceDefinitionV1Beta1Builder{
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

// Condition adds a CustomResourceDefinitionCondition objects to a CustomResourceDefinitionV1Beta1Builder.
func (c *CustomResourceDefinitionV1Beta1Builder) Condition(cond apiextv1beta1.CustomResourceDefinitionCondition) *CustomResourceDefinitionV1Beta1Builder {
	c.object.Status.Conditions = append(c.object.Status.Conditions, cond)
	return c
}

// Result returns the built CustomResourceDefinition.
func (b *CustomResourceDefinitionV1Beta1Builder) Result() *apiextv1beta1.CustomResourceDefinition {
	return b.object
}

// ObjectMeta applies functional options to the CustomResourceDefinition's ObjectMeta.
func (b *CustomResourceDefinitionV1Beta1Builder) ObjectMeta(opts ...ObjectMetaOpt) *CustomResourceDefinitionV1Beta1Builder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// CustomResourceDefinitionV1Beta1ConditionBuilder builds CustomResourceDefinitionV1Beta1Condition objects.
type CustomResourceDefinitionV1Beta1ConditionBuilder struct {
	object apiextv1beta1.CustomResourceDefinitionCondition
}

// ForCustomResourceDefinitionV1Beta1Condition is the construction for a CustomResourceDefinitionV1Beta1ConditionBuilder.
func ForCustomResourceDefinitionV1Beta1Condition() *CustomResourceDefinitionV1Beta1ConditionBuilder {
	return &CustomResourceDefinitionV1Beta1ConditionBuilder{
		object: apiextv1beta1.CustomResourceDefinitionCondition{},
	}
}

// Type sets the Condition's type.
func (c *CustomResourceDefinitionV1Beta1ConditionBuilder) Type(t apiextv1beta1.CustomResourceDefinitionConditionType) *CustomResourceDefinitionV1Beta1ConditionBuilder {
	c.object.Type = t
	return c
}

// Status sets the Condition's status.
func (c *CustomResourceDefinitionV1Beta1ConditionBuilder) Status(cs apiextv1beta1.ConditionStatus) *CustomResourceDefinitionV1Beta1ConditionBuilder {
	c.object.Status = cs
	return c
}

// Result returns the built v1beta1 CustomResourceDefinitionCondition.
func (c *CustomResourceDefinitionV1Beta1ConditionBuilder) Result() apiextv1beta1.CustomResourceDefinitionCondition {
	return c.object
}
