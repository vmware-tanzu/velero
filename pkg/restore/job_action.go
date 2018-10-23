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
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type jobAction struct {
	logger logrus.FieldLogger
}

func NewJobAction(logger logrus.FieldLogger) ItemAction {
	return &jobAction{logger: logger}
}

func (a *jobAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"jobs"},
	}, nil
}

func (a *jobAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	fieldDeletions := map[string]string{
		"spec.selector.matchLabels":     "controller-uid",
		"spec.template.metadata.labels": "controller-uid",
	}

	for k, v := range fieldDeletions {
		a.logger.Debugf("Getting %s", k)
		labels, err := collections.GetMap(obj.UnstructuredContent(), k)
		if err != nil {
			a.logger.WithError(err).Debugf("Unable to get %s", k)
		} else {
			delete(labels, v)
		}
	}

	return obj, nil, nil
}
