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

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
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
	a.logger.Debug("getting spec")
	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, nil, err
	}

	a.logger.Debug("deleting spec.NodeName")
	delete(spec, "nodeName")

	newVolumes := make([]interface{}, 0)
	a.logger.Debug("iterating over volumes")
	err = collections.ForEach(spec, "volumes", func(volume map[string]interface{}) error {
		name, err := collections.GetString(volume, "name")
		if err != nil {
			return err
		}

		a.logger.WithField("volumeName", name).Debug("Checking volume")
		if !defaultTokenRegex.MatchString(name) {
			a.logger.WithField("volumeName", name).Debug("Preserving volume")
			newVolumes = append(newVolumes, volume)
		} else {
			a.logger.WithField("volumeName", name).Debug("Excluding volume")
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	a.logger.Debug("Setting spec.volumes")
	spec["volumes"] = newVolumes

	a.logger.Debug("iterating over containers")
	err = collections.ForEach(spec, "containers", func(container map[string]interface{}) error {
		var newVolumeMounts []interface{}
		err := collections.ForEach(container, "volumeMounts", func(volumeMount map[string]interface{}) error {
			name, err := collections.GetString(volumeMount, "name")
			if err != nil {
				return err
			}

			a.logger.WithField("volumeMount", name).Debug("Checking volumeMount")
			if !defaultTokenRegex.MatchString(name) {
				a.logger.WithField("volumeMount", name).Debug("Preserving volumeMount")
				newVolumeMounts = append(newVolumeMounts, volumeMount)
			} else {
				a.logger.WithField("volumeMount", name).Debug("Excluding volumeMount")
			}

			return nil
		})
		if err != nil {
			return err
		}

		container["volumeMounts"] = newVolumeMounts

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return obj, nil, nil
}
