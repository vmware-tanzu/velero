/*
Copyright 2019, 2020 the Velero contributors.

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

// Merge combines two Result objects into one
// by appending the corresponding lists to one another.
func (r *Result) Merge(other *Result) {
	r.Cluster = append(r.Cluster, other.Cluster...)
	r.Velero = append(r.Velero, other.Velero...)
	for k, v := range other.Namespaces {
		if r.Namespaces == nil {
			r.Namespaces = make(map[string][]string)
		}
		r.Namespaces[k] = append(r.Namespaces[k], v...)
	}
}

// AddVeleroError appends an error to the provided Result's Velero list.
func (r *Result) AddVeleroError(err error) {
	r.Velero = append(r.Velero, err.Error())
}

// Add appends an error to the provided Result, either within
// the cluster-scoped list (if ns == "") or within the provided namespace's
// entry.
func (r *Result) Add(ns string, e error) {
	if ns == "" {
		r.Cluster = append(r.Cluster, e.Error())
	} else {
		if r.Namespaces == nil {
			r.Namespaces = make(map[string][]string)
		}
		r.Namespaces[ns] = append(r.Namespaces[ns], e.Error())
	}
}
