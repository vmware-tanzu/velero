/*
Copyright 2020 the Velero contributors.

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

// V1CustomResourceDefinitionBuilder builds CustomResourceDefinition objects.
type V1CustomResourceDefinitionBuilder struct {
	object *apiextv1.CustomResourceDefinition
}

// ForV1CustomResourceDefinition is the constructor for a V1CustomResourceDefinitionBuilder.
func ForV1CustomResourceDefinition(name string) *V1CustomResourceDefinitionBuilder {
	return &V1CustomResourceDefinitionBuilder{
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

// Condition adds a CustomResourceDefinitionCondition objects to a V1CustomResourceDefinitionBuilder.
func (b *V1CustomResourceDefinitionBuilder) Condition(cond apiextv1.CustomResourceDefinitionCondition) *V1CustomResourceDefinitionBuilder {
	b.object.Status.Conditions = append(b.object.Status.Conditions, cond)
	return b
}

// Version adds a CustomResourceDefinitionVersion object to a V1CustomResourceDefinitionBuilder.
func (b *V1CustomResourceDefinitionBuilder) Version(ver apiextv1.CustomResourceDefinitionVersion) *V1CustomResourceDefinitionBuilder {
	b.object.Spec.Versions = append(b.object.Spec.Versions, ver)
	return b
}

// PreserveUnknownFields sets PreserveUnknownFields on a CustomResourceDefinition.
func (b *V1CustomResourceDefinitionBuilder) PreserveUnknownFields(preserve bool) *V1CustomResourceDefinitionBuilder {
	b.object.Spec.PreserveUnknownFields = preserve
	return b
}

// Result returns the built CustomResourceDefinition.
func (b *V1CustomResourceDefinitionBuilder) Result() *apiextv1.CustomResourceDefinition {
	return b.object
}

// ObjectMeta applies functional options to the CustomResourceDefinition's ObjectMeta.
func (b *V1CustomResourceDefinitionBuilder) ObjectMeta(opts ...ObjectMetaOpt) *V1CustomResourceDefinitionBuilder {
	for _, opt := range opts {
		opt(b.object)
	}

	return b
}

// V1CustomResourceDefinitionConditionBuilder builds CustomResourceDefinitionCondition objects.
type V1CustomResourceDefinitionConditionBuilder struct {
	object apiextv1.CustomResourceDefinitionCondition
}

// ForV1V1CustomResourceDefinitionConditionBuilder is the constructor for a V1CustomResourceDefinitionConditionBuilder.
func ForV1CustomResourceDefinitionCondition() *V1CustomResourceDefinitionConditionBuilder {
	return &V1CustomResourceDefinitionConditionBuilder{
		object: apiextv1.CustomResourceDefinitionCondition{},
	}
}

// Type sets the Condition's type.
func (c *V1CustomResourceDefinitionConditionBuilder) Type(t apiextv1.CustomResourceDefinitionConditionType) *V1CustomResourceDefinitionConditionBuilder {
	c.object.Type = t
	return c
}

// Status sets the Condition's status.
func (c *V1CustomResourceDefinitionConditionBuilder) Status(cs apiextv1.ConditionStatus) *V1CustomResourceDefinitionConditionBuilder {
	c.object.Status = cs
	return c
}

// Result returns the built CustomResourceDefinitionCondition.
func (b *V1CustomResourceDefinitionConditionBuilder) Result() apiextv1.CustomResourceDefinitionCondition {
	return b.object
}

// V1CustomResourceDefinitionVersionBuilder builds CustomResourceDefinitionVersion objects.
type V1CustomResourceDefinitionVersionBuilder struct {
	object apiextv1.CustomResourceDefinitionVersion
}

// ForV1CustomResourceDefinitionVersion is the constructor for a V1CustomResourceDefinitionVersionBuilder.
func ForV1CustomResourceDefinitionVersion(name string) *V1CustomResourceDefinitionVersionBuilder {
	return &V1CustomResourceDefinitionVersionBuilder{
		object: apiextv1.CustomResourceDefinitionVersion{Name: name},
	}
}

// Served sets the Served field on a CustomResourceDefinitionVersion.
func (b *V1CustomResourceDefinitionVersionBuilder) Served(s bool) *V1CustomResourceDefinitionVersionBuilder {
	b.object.Served = s
	return b
}

// Storage sets the Storage field on a CustomResourceDefinitionVersion.
func (b *V1CustomResourceDefinitionVersionBuilder) Storage(s bool) *V1CustomResourceDefinitionVersionBuilder {
	b.object.Storage = s
	return b
}

func (b *V1CustomResourceDefinitionVersionBuilder) Schema(s *apiextv1.JSONSchemaProps) *V1CustomResourceDefinitionVersionBuilder {
	if b.object.Schema == nil {
		b.object.Schema = new(apiextv1.CustomResourceValidation)
	}
	b.object.Schema.OpenAPIV3Schema = s
	return b
}

// Result returns the built CustomResourceDefinitionVersion.
func (b *V1CustomResourceDefinitionVersionBuilder) Result() apiextv1.CustomResourceDefinitionVersion {
	return b.object
}
