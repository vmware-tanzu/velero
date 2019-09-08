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

package features

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

type featureFlagSet struct {
	mut sync.Mutex
	set sets.String
}

// featureFlags will store all the flags for this process until NewFeatureFlagSet is called.
var featureFlags featureFlagSet

// Enabled returns True if a specified flag is enabled.
func Enabled(name string) bool {
	featureFlags.mut.Lock()
	defer featureFlags.mut.Unlock()

	return featureFlags.set.Has(name)
}

// Enable adds a given slice of feature names to the current feature list.
func Enable(names ...string) {
	featureFlags.mut.Lock()
	defer featureFlags.mut.Unlock()

	featureFlags.set.Insert(names...)
}

// Disable removes all feature flags in a given slice from the current feature list.
func Disable(names ...string) {
	featureFlags.mut.Lock()
	defer featureFlags.mut.Unlock()

	featureFlags.set.Delete(names...)
}

// All returns enabled features as a slice of strings.
func All() []string {
	return featureFlags.set.List()
}

// Serialize returns all features as a comma-separated string.
func Serialize() string {
	return strings.Join(All(), ",")
}

// NewFeaturesetet initializes and populates a new FeatureFlagSet.
// This must be called to properly initialize the set for tracking flags.
// It is also useful for selectively controlling flags during tests.
func NewFeatureFlagSet(flags ...string) {
	featureFlags = featureFlagSet{
		mut: sync.Mutex{},
		set: sets.NewString(flags...),
	}
}
