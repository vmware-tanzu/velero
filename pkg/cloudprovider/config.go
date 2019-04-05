/*
Copyright 2019 the Velero contributors.

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

package cloudprovider

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateObjectStoreConfigKeys ensures that an object store's config
// is valid by making sure each `config` key is in the `validKeys` list.
// The special key "bucket" is always considered valid.
func ValidateObjectStoreConfigKeys(config map[string]string, validKeys ...string) error {
	// `bucket` is automatically added to all object store config by
	// velero, so add it as a valid key.
	return validateConfigKeys(config, append(validKeys, "bucket")...)
}

// ValidateVolumeSnapshotterConfigKeys ensures that a volume snapshotter's
// config is valid by making sure each `config` key is in the `validKeys` list.
func ValidateVolumeSnapshotterConfigKeys(config map[string]string, validKeys ...string) error {
	return validateConfigKeys(config, validKeys...)
}

func validateConfigKeys(config map[string]string, validKeys ...string) error {
	validKeysSet := sets.NewString(validKeys...)

	var invalidKeys []string
	for k := range config {
		if !validKeysSet.Has(k) {
			invalidKeys = append(invalidKeys, k)
		}
	}

	if len(invalidKeys) > 0 {
		return errors.Errorf("config has invalid keys %v; valid keys are %v", invalidKeys, validKeys)
	}

	return nil
}
