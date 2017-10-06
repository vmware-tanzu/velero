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
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type jobRestorer struct {
	logger *logrus.Logger
}

var _ ResourceRestorer = &jobRestorer{}

func NewJobRestorer(logger *logrus.Logger) ResourceRestorer {
	return &jobRestorer{
		logger: logger,
	}
}

func (r *jobRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (r *jobRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	r.logger.Debug("resetting metadata and status")
	_, err := resetMetadataAndStatus(obj, true)
	if err != nil {
		return nil, nil, err
	}

	fieldDeletions := map[string]string{
		"spec.selector.matchLabels":     "controller-uid",
		"spec.template.metadata.labels": "controller-uid",
	}

	for k, v := range fieldDeletions {
		r.logger.Debugf("Getting %s", k)
		labels, err := collections.GetMap(obj.UnstructuredContent(), k)
		if err != nil {
			r.logger.WithError(err).Debugf("Unable to get %s", k)
		} else {
			delete(labels, v)
		}
	}

	return obj, nil, nil
}

func (r *jobRestorer) Wait() bool {
	return false
}

func (r *jobRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
