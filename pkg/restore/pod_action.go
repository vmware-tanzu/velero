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
	"strings"

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

func (a *podAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	a.logger.Debug("getting spec")
	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, nil, err
	}

	a.logger.Debug("deleting spec.NodeName")
	delete(spec, "nodeName")

	// if there are no volumes, then there can't be any volume mounts, so we're done.
	if !collections.Exists(spec, "volumes") {
		return obj, nil, nil
	}

	serviceAccountName, err := collections.GetString(spec, "serviceAccountName")
	if err != nil {
		return nil, nil, err
	}
	prefix := serviceAccountName + "-token-"

	// remove the service account token from volumes
	a.logger.Debug("iterating over volumes")
	if err := removeItemsWithNamePrefix(spec, "volumes", prefix, a.logger); err != nil {
		return nil, nil, err
	}

	// remove the service account token volume mount from all containers
	a.logger.Debug("iterating over containers")
	if err := removeVolumeMounts(spec, "containers", prefix, a.logger); err != nil {
		return nil, nil, err
	}

	if !collections.Exists(spec, "initContainers") {
		return obj, nil, nil
	}

	// remove the service account token volume mount from all init containers
	a.logger.Debug("iterating over init containers")
	if err := removeVolumeMounts(spec, "initContainers", prefix, a.logger); err != nil {
		return nil, nil, err
	}

	return obj, nil, nil
}

// removeItemsWithNamePrefix iterates through the collection stored at 'key' in 'unstructuredObj'
// and removes any item that has a name that starts with 'prefix'.
func removeItemsWithNamePrefix(unstructuredObj map[string]interface{}, key, prefix string, log logrus.FieldLogger) error {
	var preservedItems []interface{}

	if err := collections.ForEach(unstructuredObj, key, func(item map[string]interface{}) error {
		name, err := collections.GetString(item, "name")
		if err != nil {
			return err
		}

		singularKey := strings.TrimSuffix(key, "s")
		log := log.WithField(singularKey, name)

		log.Debug("Checking " + singularKey)
		switch {
		case strings.HasPrefix(name, prefix):
			log.Debug("Excluding ", singularKey)
		default:
			log.Debug("Preserving ", singularKey)
			preservedItems = append(preservedItems, item)
		}

		return nil
	}); err != nil {
		return err
	}

	unstructuredObj[key] = preservedItems
	return nil
}

// removeVolumeMounts iterates through a slice of containers stored at 'containersKey' in
// 'podSpec' and removes any volume mounts with a name starting with 'prefix'.
func removeVolumeMounts(podSpec map[string]interface{}, containersKey, prefix string, log logrus.FieldLogger) error {
	return collections.ForEach(podSpec, containersKey, func(container map[string]interface{}) error {
		if !collections.Exists(container, "volumeMounts") {
			return nil
		}

		return removeItemsWithNamePrefix(container, "volumeMounts", prefix, log)
	})
}
