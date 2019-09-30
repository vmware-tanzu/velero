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

package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

func Test_resourceKey(t *testing.T) {
	tests := []struct {
		resource metav1.Object
		want     string
	}{
		{resource: builder.ForPod("default", "test").Result(), want: "v1/Pod"},
		{resource: builder.ForDeployment("default", "test").Result(), want: "apps/v1/Deployment"},
		{resource: builder.ForPersistentVolume("test").Result(), want: "v1/PersistentVolume"},
		{resource: builder.ForRole("default", "test").Result(), want: "rbac.authorization.k8s.io/v1/Role"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			content, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.resource)
			unstructured := &unstructured.Unstructured{Object: content}
			assert.Equal(t, tt.want, resourceKey(unstructured))
		})
	}
}
