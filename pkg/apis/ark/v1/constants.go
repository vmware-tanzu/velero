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

package v1

const (
	// DefaultNamespace is the Kubernetes namespace that is used by default for
	// the Ark server and API objects.
	DefaultNamespace = "heptio-ark"

	// ResourcesDir is a top-level directory expected in backups which contains sub-directories
	// for each resource type in the backup.
	ResourcesDir = "resources"

	// RestoreLabelKey is the label key that's applied to all resources that
	// are created during a restore. This is applied for ease of identification
	// of restored resources. The value will be the restore's name.
	//
	// This label is DEPRECATED as of v0.10 and will be removed entirely as of
	// v1.0 and replaced with RestoreNameLabel ("ark.heptio.com/restore-name").
	RestoreLabelKey = "ark-restore"

	// ClusterScopedDir is the name of the directory containing cluster-scoped
	// resources within an Ark backup.
	ClusterScopedDir = "cluster"

	// NamespaceScopedDir is the name of the directory containing namespace-scoped
	// resource within an Ark backup.
	NamespaceScopedDir = "namespaces"
)
