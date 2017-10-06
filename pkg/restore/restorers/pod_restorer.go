/*
Copyright 2017 Heptio Inc.

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

package restorers

import (
	"regexp"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type podRestorer struct {
	logger *logrus.Logger
}

var _ ResourceRestorer = &podRestorer{}

func NewPodRestorer(logger *logrus.Logger) ResourceRestorer {
	return &podRestorer{
		logger: logger,
	}
}

func (nsr *podRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

var (
	defaultTokenRegex = regexp.MustCompile("default-token-.*")
)

func (r *podRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	r.logger.Debug("resetting metadata and status")
	_, err := resetMetadataAndStatus(obj, true)
	if err != nil {
		return nil, nil, err
	}

	r.logger.Debug("getting spec")
	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, nil, err
	}

	r.logger.Debug("deleting spec.NodeName")
	delete(spec, "nodeName")

	newVolumes := make([]interface{}, 0)
	r.logger.Debug("iterating over volumes")
	err = collections.ForEach(spec, "volumes", func(volume map[string]interface{}) error {
		name, err := collections.GetString(volume, "name")
		if err != nil {
			return err
		}

		r.logger.WithField("volumeName", name).Debug("Checking volume")
		if !defaultTokenRegex.MatchString(name) {
			r.logger.WithField("volumeName", name).Debug("Preserving volume")
			newVolumes = append(newVolumes, volume)
		} else {
			r.logger.WithField("volumeName", name).Debug("Excluding volume")
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	r.logger.Debug("Setting spec.volumes")
	spec["volumes"] = newVolumes

	r.logger.Debug("iterating over containers")
	err = collections.ForEach(spec, "containers", func(container map[string]interface{}) error {
		var newVolumeMounts []interface{}
		err := collections.ForEach(container, "volumeMounts", func(volumeMount map[string]interface{}) error {
			name, err := collections.GetString(volumeMount, "name")
			if err != nil {
				return err
			}

			r.logger.WithField("volumeMount", name).Debug("Checking volumeMount")
			if !defaultTokenRegex.MatchString(name) {
				r.logger.WithField("volumeMount", name).Debug("Preserving volumeMount")
				newVolumeMounts = append(newVolumeMounts, volumeMount)
			} else {
				r.logger.WithField("volumeMount", name).Debug("Excluding volumeMount")
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

func (nsr *podRestorer) Wait() bool {
	return false
}

func (nsr *podRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
