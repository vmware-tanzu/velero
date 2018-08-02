/*
Copyright 2017 the Heptio Ark contributors.

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

package restore

import (
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

type podAction struct {
	logger logrus.FieldLogger
}

func NewPodAction(logger logrus.FieldLogger) ItemAction {
	return &podAction{logger: logger}
}

func (a *podAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

var (
	defaultTokenRegex = regexp.MustCompile("default-token-.*")
)

func (a *podAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	unstructured.RemoveNestedField(obj.UnstructuredContent(), "spec", "nodeName")

	volumes, _, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "volumes")
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	if len(volumes) > 0 {
		var nonDefaultTokenVolumes []interface{}
		for i, obj := range volumes {
			volume, ok := obj.(map[string]interface{})
			if !ok {
				return nil, nil, errors.Errorf("expected .spec.volumes[%d] to be of type map[string]interface{}, was %T", i, obj)
			}

			name, _, err := unstructured.NestedString(volume, "name")
			if err != nil {
				return nil, nil, errors.WithStack(err)
			}

			if !defaultTokenRegex.MatchString(name) {
				nonDefaultTokenVolumes = append(nonDefaultTokenVolumes, volume)
			}
		}
		if err := unstructured.SetNestedSlice(obj.UnstructuredContent(), nonDefaultTokenVolumes, "spec", "volumes"); err != nil {
			return nil, nil, errors.WithStack(err)
		}
	}

	containers, _, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "containers")
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	for i, obj := range containers {
		container, ok := obj.(map[string]interface{})
		if !ok {
			return nil, nil, errors.Errorf("expected .spec.containers[%d] to be of type map[string]interface{}, was %T", i, obj)
		}

		volumeMounts, _, err := unstructured.NestedSlice(container, "volumeMounts")
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		var nonDefaultTokenVolumeMounts []interface{}
		for j, obj := range volumeMounts {
			volumeMount, ok := obj.(map[string]interface{})
			if !ok {
				return nil, nil, errors.Errorf("expected .spec.volumes[%d].volumeMounts[%d] to be of type map[string]interface{}, was %T", i, j, obj)
			}

			name, _, err := unstructured.NestedString(volumeMount, "name")
			if err != nil {
				return nil, nil, errors.WithStack(err)
			}

			if !defaultTokenRegex.MatchString(name) {
				nonDefaultTokenVolumeMounts = append(nonDefaultTokenVolumeMounts, volumeMount)
			}
		}
		container["volumeMounts"] = nonDefaultTokenVolumeMounts
	}

	if err := unstructured.SetNestedField(obj.UnstructuredContent(), containers, "spec", "containers"); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return obj, nil, nil
}
