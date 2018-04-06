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
	return &podAction{
		logger: logger,
	}
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
	a.logger.Debug("deleting spec.NodeName")
	unstructured.RemoveNestedField(obj.UnstructuredContent(), "spec", "nodeName")

	var newVolumes []interface{}

	a.logger.Debug("iterating over volumes")
	volumes, found := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "volumes")
	if !found {
		return nil, nil, errors.New("unable to get spec.volumes")
	}

	for _, volume := range volumes {
		volumeMap, ok := volume.(map[string]interface{})
		if !ok {
			return nil, nil, errors.New("unable to convert volume to a map[string]interface{}")
		}

		name, found := unstructured.NestedString(volumeMap, "name")
		if !found {
			return nil, nil, errors.New("unable to get volume name")
		}

		a.logger.WithField("volumeName", name).Debug("Checking volume")
		if !defaultTokenRegex.MatchString(name) {
			a.logger.WithField("volumeName", name).Debug("Preserving volume")
			newVolumes = append(newVolumes, volume)
		} else {
			a.logger.WithField("volumeName", name).Debug("Excluding volume")
		}
	}

	a.logger.Debug("Setting spec.volumes")
	unstructured.SetNestedField(obj.UnstructuredContent(), newVolumes, "spec", "volumes")

	a.logger.Debug("iterating over containers")
	containers, found := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "containers")
	if !found {
		return nil, nil, errors.New("unable to get spec.containers")
	}
	for _, container := range containers {
		containerMap, ok := container.(map[string]interface{})
		if !ok {
			return nil, nil, errors.New("unable to convert container to a map[string]interface{}")
		}

		var newVolumeMounts []interface{}
		volumeMounts, found := unstructured.NestedSlice(containerMap, "volumeMounts")
		if !found {
			return nil, nil, errors.New("unable to get volume mounts")
		}
		for _, volumeMount := range volumeMounts {
			volumeMountMap, ok := volumeMount.(map[string]interface{})
			if !ok {
				return nil, nil, errors.New("unable to convert volume mount to a map[string]interface{}")
			}

			name, found := unstructured.NestedString(volumeMountMap, "name")
			if !found {
				return nil, nil, errors.New("unable to get volume mount name")
			}

			a.logger.WithField("volumeMount", name).Debug("Checking volumeMount")
			if !defaultTokenRegex.MatchString(name) {
				a.logger.WithField("volumeMount", name).Debug("Preserving volumeMount")
				newVolumeMounts = append(newVolumeMounts, volumeMountMap)
			} else {
				a.logger.WithField("volumeMount", name).Debug("Excluding volumeMount")
			}
		}

		unstructured.SetNestedField(containerMap, newVolumeMounts, "volumeMounts")
	}

	unstructured.SetNestedField(obj.UnstructuredContent(), containers, "spec", "containers")

	return obj, nil, nil
}
