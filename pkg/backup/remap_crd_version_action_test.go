/*
Copyright 2020 the Velero contributors.

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
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestRemapCRDVersionAction(t *testing.T) {
	backup := &v1.Backup{}
	a := NewRemapCRDVersionAction(velerotest.NewLogger())

	t.Run("Test a v1 CRD without any Schema information", func(t *testing.T) {
		b := builder.ForCustomResourceDefinition("test.velero.io")
		// Set a version that does not include and schema information.
		b.Version(builder.ForCustomResourceDefinitionVersion("v1").Served(true).Storage(true).Result())
		c := b.Result()
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		require.NoError(t, err)

		item, _, err := a.Execute(&unstructured.Unstructured{Object: obj}, backup)
		require.NoError(t, err)
		assert.Equal(t, "apiextensions.k8s.io/v1beta1", item.UnstructuredContent()["apiVersion"])
	})

	t.Run("Test a v1 CRD with a NonStructuralSchema Condition", func(t *testing.T) {
		b := builder.ForCustomResourceDefinition("test.velero.io")
		b.Condition(builder.ForCustomResourceDefinitionCondition().Type(apiextv1.NonStructuralSchema).Result())
		c := b.Result()
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		require.NoError(t, err)

		item, _, err := a.Execute(&unstructured.Unstructured{Object: obj}, backup)
		require.NoError(t, err)
		assert.Equal(t, "apiextensions.k8s.io/v1beta1", item.UnstructuredContent()["apiVersion"])
	})
}
