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
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type jobRestorer struct{}

var _ ResourceRestorer = &jobRestorer{}

func NewJobRestorer() ResourceRestorer {
	return &jobRestorer{}
}

func (r *jobRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (r *jobRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	glog.V(4).Infof("resetting metadata and status")
	_, err := resetMetadataAndStatus(obj, true)
	if err != nil {
		return nil, nil, err
	}

	glog.V(4).Infof("getting spec.selector.matchLabels")
	matchLabels, err := collections.GetMap(obj.UnstructuredContent(), "spec.selector.matchLabels")
	if err != nil {
		glog.V(4).Infof("unable to get spec.selector.matchLabels: %v", err)
	} else {
		delete(matchLabels, "controller-uid")
	}

	templateLabels, err := collections.GetMap(obj.UnstructuredContent(), "spec.template.metadata.labels")
	if err != nil {
		glog.V(4).Infof("unable to get spec.template.metadata.labels: %v", err)
	} else {
		delete(templateLabels, "controller-uid")
	}

	return obj, nil, nil
}

func (r *jobRestorer) Wait() bool {
	return false
}

func (r *jobRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
