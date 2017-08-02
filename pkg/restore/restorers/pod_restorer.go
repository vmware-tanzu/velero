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

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type podRestorer struct{}

var _ ResourceRestorer = &podRestorer{}

func NewPodRestorer() ResourceRestorer {
	return &podRestorer{}
}

func (nsr *podRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

var (
	defaultTokenRegex = regexp.MustCompile("default-token-.*")
)

func (nsr *podRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error) {
	glog.V(4).Infof("resetting metadata and status")
	_, err := resetMetadataAndStatus(obj, true)
	if err != nil {
		return nil, err
	}

	glog.V(4).Infof("getting spec")
	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, err
	}

	glog.V(4).Infof("deleting spec.NodeName")
	delete(spec, "nodeName")

	newVolumes := make([]interface{}, 0)
	glog.V(4).Infof("iterating over volumes")
	err = collections.ForEach(spec, "volumes", func(volume map[string]interface{}) error {
		name, err := collections.GetString(volume, "name")
		if err != nil {
			return err
		}

		glog.V(4).Infof("checking volume with name %q", name)

		if !defaultTokenRegex.MatchString(name) {
			glog.V(4).Infof("preserving volume")
			newVolumes = append(newVolumes, volume)
		} else {
			glog.V(4).Infof("excluding volume")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	glog.V(4).Infof("setting spec.volumes")
	spec["volumes"] = newVolumes

	glog.V(4).Infof("iterating over containers")
	err = collections.ForEach(spec, "containers", func(container map[string]interface{}) error {
		var newVolumeMounts []interface{}
		err := collections.ForEach(container, "volumeMounts", func(volumeMount map[string]interface{}) error {
			name, err := collections.GetString(volumeMount, "name")
			if err != nil {
				return err
			}

			glog.V(4).Infof("checking volumeMount with name %q", name)

			if !defaultTokenRegex.MatchString(name) {
				glog.V(4).Infof("preserving volumeMount")
				newVolumeMounts = append(newVolumeMounts, volumeMount)
			} else {
				glog.V(4).Infof("excluding volumeMount")
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
		return nil, err
	}

	return obj, nil
}

func (nsr *podRestorer) Wait() bool {
	return false
}

func (nsr *podRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
