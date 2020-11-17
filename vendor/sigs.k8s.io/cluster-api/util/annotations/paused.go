/*
Copyright 2020 The Kubernetes Authors.

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

package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// IsPaused returns true if the Cluster is paused or the object has the `paused` annotation.
func IsPaused(cluster *clusterv1.Cluster, o metav1.Object) bool {
	if cluster.Spec.Paused {
		return true
	}
	return HasPausedAnnotation(o)
}

// HasPausedAnnotation returns true if the object has the `paused` annotation.
func HasPausedAnnotation(o metav1.Object) bool {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[clusterv1.PausedAnnotation]
	return ok
}
