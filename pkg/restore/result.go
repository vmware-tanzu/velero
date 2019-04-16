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

package restore

// Result is a collection of messages that were generated during
// execution of a restore. This will typically store either
// warning or error messages.
type Result struct {
	// Velero is a slice of messages related to the operation of Velero
	// itself (for example, messages related to connecting to the
	// cloud, reading a backup file, etc.)
	Velero []string `json:"velero,omitempty"`

	// Cluster is a slice of messages related to restoring cluster-
	// scoped resources.
	Cluster []string `json:"cluster,omitempty"`

	// Namespaces is a map of namespace name to slice of messages
	// related to restoring namespace-scoped resources.
	Namespaces map[string][]string `json:"namespaces,omitempty"`
}
