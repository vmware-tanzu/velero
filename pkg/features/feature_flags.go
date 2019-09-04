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

import "k8s.io/apimachinery/pkg/util/sets"

type FeatureFlagSet struct {
	flags sets.String
}

type Flags interface {
	// Enabled reports whether or not the specified flag is found.
	Enabled(name string) bool

	// Enable adds the specified flags to the list of enabled flags.
	Enable(names ...string)

	// All returns all enabled features
	All() []string
}

func (f *FeatureFlagSet) Enabled(name string) bool {
	return f.flags.Has(name)
}

func (f *FeatureFlagSet) Enable(names ...string) {
	f.flags.Insert(names...)
}

func (f *FeatureFlagSet) All() []string {
	return f.flags.List()
}

// NewFeatureFlagSet initializes and populates a new FeatureFlagSet
func NewFeatureFlagSet(flags ...string) *FeatureFlagSet {
	return &FeatureFlagSet{
		flags: sets.NewString(flags...),
	}
}
