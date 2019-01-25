/*
Copyright 2018 the Heptio Ark contributors.

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

package install

import (
	"fmt"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

// CRDs returns a list of the CRD types for all of the required Velero CRDs
func CRDs() []*apiextv1beta1.CustomResourceDefinition {
	var crds []*apiextv1beta1.CustomResourceDefinition

	for kind, typeInfo := range velerov1api.CustomResources() {
		crds = append(crds, crd(kind, typeInfo.PluralName))
	}

	return crds
}

func crd(kind, plural string) *apiextv1beta1.CustomResourceDefinition {
	return &apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, velerov1api.GroupName),
		},
		Spec: apiextv1beta1.CustomResourceDefinitionSpec{
			Group:   velerov1api.GroupName,
			Version: velerov1api.SchemeGroupVersion.Version,
			Scope:   apiextv1beta1.NamespaceScoped,
			Names: apiextv1beta1.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   kind,
			},
		},
	}
}
