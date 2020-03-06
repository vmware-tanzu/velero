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

package backup

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// RemapCRDVersionAction inspects CustomResourceDefinition and decides if it is a v1
// CRD that needs to be backed up as v1beta1.
type RemapCRDVersionAction struct {
	logger logrus.FieldLogger
}

// NewRemapCRDVersionAction instantiates a new RemapCRDVersionAction plugin.
func NewRemapCRDVersionAction(logger logrus.FieldLogger) *RemapCRDVersionAction {
	return &RemapCRDVersionAction{logger: logger}
}

// AppliesTo selects the resources the plugin should run against. In this case, CustomResourceDefinitions.
func (a *RemapCRDVersionAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"customresourcedefinition.apiextensions.k8s.io"},
	}, nil
}

// Execute executes logic necessary to check a CustomResourceDefinition and inspect it for characteristics that necessitate saving it as v1beta1 instead of v1.
func (a *RemapCRDVersionAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.logger.Info("Executing RemapCRDVersionAction")

	// This plugin is only relevant for CRDs retrieved from the v1 endpoint that were installed via the v1beta1
	// endpoint, so we can exit immediately if the resource in question isn't v1.
	apiVersion, ok, err := unstructured.NestedString(item.UnstructuredContent(), "apiVersion")
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to read apiVersion from CRD")
	}
	if ok && apiVersion != "apiextensions.k8s.io/v1" {
		a.logger.Info("Exiting RemapCRDVersionAction, CRD is not v1")
		return item, nil, nil
	}

	// We've got a v1 CRD, so proceed.
	var crd apiextv1.CustomResourceDefinition

	// Do not use runtime.DefaultUnstructuredConverter.FromUnstructured here because it has a bug when converting integers/whole
	// numbers in float fields (https://github.com/kubernetes/kubernetes/issues/87675).
	// Using JSON as a go-between avoids this issue, without adding a bunch of type conversion by using unstructured helper functions
	// to inspect the fields we want to look at.
	js, err := json.Marshal(item.UnstructuredContent())
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert unstructured item to JSON")
	}

	if err = json.Unmarshal(js, &crd); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert JSON to CRD Go type")
	}

	log := a.logger.WithField("plugin", "RemapCRDVersionAction").WithField("CRD", crd.Name)

	// Looking for 1 version should be enough to tell if it's a v1beta1 CRD, as all v1beta1 CRD versions share the same schema.
	// v1 CRDs can have different schemas per version
	// The silently upgraded versions will often have a `versions` entry that looks like this:
	//   versions:
	//   - name: v1
	//     served:  true
	//     storage: true
	// This is acceptable when re-submitted to a v1beta1 endpoint on restore.
	if len(crd.Spec.Versions) > 0 {
		if crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
			log.Debug("CRD is a candidate for v1beta1 backup")

			if err := setV1beta1Version(item); err != nil {
				return nil, nil, err
			}
		}
	}

	// If the NonStructuralSchema condition was applied, be sure to back it up as v1beta1.
	for _, c := range crd.Status.Conditions {
		if c.Type == apiextv1.NonStructuralSchema {
			log.Debug("CRD is a non-structural schema")

			if err := setV1beta1Version(item); err != nil {
				return nil, nil, err
			}

			break
		}
	}

	return item, nil, nil
}

// setV1beta1Version updates the apiVersion field of an Unstructured CRD to be the v1beta1 string instead of v1.
func setV1beta1Version(u runtime.Unstructured) error {
	// Since we can't manipulate an Unstructured's Object map directly, get a copy to manipulate before setting it back
	tempMap := u.UnstructuredContent()
	if err := unstructured.SetNestedField(tempMap, "apiextensions.k8s.io/v1beta1", "apiVersion"); err != nil {
		return errors.Wrap(err, "unable to set apiversion to v1beta1")
	}
	u.SetUnstructuredContent(tempMap)
	return nil
}
