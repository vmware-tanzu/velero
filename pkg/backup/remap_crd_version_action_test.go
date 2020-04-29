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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextfakes "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

// TODO: How do we get the elasticsearch & crossplane CRDs in here to run w/o a cluster?
func TestRemapCRDVersionAction(t *testing.T) {
	backup := &v1.Backup{}
	clientset := apiextfakes.NewSimpleClientset()
	betaClient := clientset.ApiextensionsV1beta1().CustomResourceDefinitions()

	// build a v1beta1 CRD with the same name and add it to the fake client that the plugin is going to call.
	// keep the same one for all 3 tests, since there's little value in recreating it
	b := builder.ForCustomResourceDefinition("test.velero.io")
	c := b.Result()
	_, err := betaClient.Create(c)
	require.NoError(t, err)

	a := NewRemapCRDVersionAction(velerotest.NewLogger(), betaClient)

	t.Run("Test a v1 CRD without any Schema information", func(t *testing.T) {
		b := builder.ForV1CustomResourceDefinition("test.velero.io")
		// Set a version that does not include and schema information.
		b.Version(builder.ForV1CustomResourceDefinitionVersion("v1").Served(true).Storage(true).Result())
		c := b.Result()

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		require.NoError(t, err)

		// Execute the plugin, which will call the fake client
		item, _, err := a.Execute(&unstructured.Unstructured{Object: obj}, backup)
		require.NoError(t, err)
		assert.Equal(t, "apiextensions.k8s.io/v1beta1", item.UnstructuredContent()["apiVersion"])
	})

	t.Run("Test a v1 CRD with a NonStructuralSchema Condition", func(t *testing.T) {
		b := builder.ForV1CustomResourceDefinition("test.velero.io")
		b.Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.NonStructuralSchema).Result())
		c := b.Result()
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		require.NoError(t, err)

		item, _, err := a.Execute(&unstructured.Unstructured{Object: obj}, backup)
		require.NoError(t, err)
		assert.Equal(t, "apiextensions.k8s.io/v1beta1", item.UnstructuredContent()["apiVersion"])
	})

	t.Run("Having an integer on a float64 field should work (issue 2319)", func(t *testing.T) {
		b := builder.ForV1CustomResourceDefinition("test.velero.io")
		// 5 here is just an int value, it could be any other whole number.
		schema := builder.ForJSONSchemaPropsBuilder().Maximum(5).Result()
		b.Version(builder.ForV1CustomResourceDefinitionVersion("v1").Served(true).Storage(true).Schema(schema).Result())
		c := b.Result()

		// Marshall in and out of JSON because the problem doesn't manifest when we use ToUnstructured directly
		// This should simulate the JSON passing over the wire in an HTTP request/response with a dynamic client
		js, err := json.Marshal(c)
		require.NoError(t, err)

		var u unstructured.Unstructured
		err = json.Unmarshal(js, &u)
		require.NoError(t, err)

		_, _, err = a.Execute(&u, backup)
		require.NoError(t, err)
	})
}
