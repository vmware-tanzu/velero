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
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type persistentVolumeClaimRestorer struct{}

var _ ResourceRestorer = &persistentVolumeClaimRestorer{}

func NewPersistentVolumeClaimRestorer() ResourceRestorer {
	return &persistentVolumeClaimRestorer{}
}

func (sr *persistentVolumeClaimRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (sr *persistentVolumeClaimRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	res, err := resetMetadataAndStatus(obj, true)

	return res, nil, err
}

func (sr *persistentVolumeClaimRestorer) Wait() bool {
	return true
}

func (sr *persistentVolumeClaimRestorer) Ready(obj runtime.Unstructured) bool {
	phase, err := collections.GetString(obj.UnstructuredContent(), "status.phase")

	return err == nil && phase == "Bound"
}
