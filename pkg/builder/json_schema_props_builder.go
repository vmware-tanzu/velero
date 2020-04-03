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
)

// JSONSchemaPropsBuilder builds JSONSchemaProps objects.
type JSONSchemaPropsBuilder struct {
	object *apiextv1.JSONSchemaProps
}

// ForJSONSchemaPropsBuilder is the constructor for a JSONSchemaPropsBuilder.
func ForJSONSchemaPropsBuilder() *JSONSchemaPropsBuilder {
	return &JSONSchemaPropsBuilder{
		object: &apiextv1.JSONSchemaProps{},
	}
}

// Maximum sets the Maximum field on a JSONSchemaPropsBuilder object.
func (b *JSONSchemaPropsBuilder) Maximum(f float64) *JSONSchemaPropsBuilder {
	b.object.Maximum = &f
	return b
}

// Result returns the built object.
func (b *JSONSchemaPropsBuilder) Result() *apiextv1.JSONSchemaProps {
	return b.object
}
