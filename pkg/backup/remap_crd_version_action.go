/*
Copyright 2017 the Velero contributors.

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

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// RemapCRDVersionAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type RemapCRDVersionAction struct {
	logger logrus.FieldLogger
}

func NewRemapCRDVersionAction(logger logrus.FieldLogger) *RemapCRDVersionAction {
	return &RemapCRDVersionAction{logger: logger}
}

func (a *RemapCRDVersionAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"customresourcedefinition.apiextensions.k8s.io"},
	}, nil
}

// TODO, if this works
func (a *RemapCRDVersionAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.logger.Info("Executing RemapCRDVersionAction")

	var crd apiextv1.CustomResourceDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &crd); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert unstructured item to CRD")
	}

	log := a.logger.WithField("plugin", "RemapCRDVersionAction").WithField("CRD", crd.Name)

	// TODO: Is it possible or likely that a CRD had multiple versions without a Schema?
	if len(crd.Spec.Versions) == 1 && crd.Spec.Versions[0].Schema == nil {
		log.Debug("CRD is a candidate for v1beta1 backup")

		// Two solutions are presented here, and they both appear to work. I've uncommented them both for readability; they won't necessarily compile this way.

		// Solution 1: create empty structs where they're required, so that the definition is technically conformant.
		// Pros:
		//   - Compatible with the existing restore plugin for x-preserve-unknown-fields, which is necessary on v1 without a schema
		//   - "upgrading" the CRD to v1 keeps the backup valid for longer, across more versions of Kubernetes
		// Cons:
		//   - Even though the structs are empty, they're not taking the data as it was.
		crd.Spec.Versions[0].Schema = new(apiextv1.CustomResourceValidation)
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema = new(apiextv1.JSONSchemaProps)

		newItem, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&crd)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to convert crd to unstructured item")
		}

		return &unstructured.Unstructured{Object: newItem}, nil, nil
	}

	// Solution 2: Change the API version that's retained in the backup so that on restore, it will be restored as v1beta1.
	// Pros:
	//   - Relatively simple; just changing a string
	//   - Data is the same as what's in the source Kubernetes cluster - the only reason we got it as a v1 instance was due to our usage of the preferred version endpoint.
	// Cons:
	//   - Data in the backup won't be valid for as long - v1beta1 CRDs will have to be upgraded sooner rather than later - but is that Velero's problem?

	// Since we can't manipulate an Unstructured's Object map directly, get a copy to manipulate before setting it back
	tempMap := item.UnstructuredContent()

	if err := unstructured.SetNestedField(tempMap, "apiextensions.k8s.io/v1beta1", "apiVersion"); err != nil {
		return nil, nil, errors.Wrap(err, "unable to set apiversion to v1beta1")
	}
	item.SetUnstructuredContent(tempMap)
	}

	return item, nil, nil
}
