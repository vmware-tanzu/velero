/*
Copyright 2018 the Heptio Ark contributors.

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

package kuberesource

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ClusterRoleBindings    = schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings"}
	ClusterRoles           = schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}
	Jobs                   = schema.GroupResource{Group: "batch", Resource: "jobs"}
	Namespaces             = schema.GroupResource{Group: "", Resource: "namespaces"}
	PersistentVolumeClaims = schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}
	PersistentVolumes      = schema.GroupResource{Group: "", Resource: "persistentvolumes"}
	Pods                   = schema.GroupResource{Group: "", Resource: "pods"}
	ServiceAccounts        = schema.GroupResource{Group: "", Resource: "serviceaccounts"}
)
