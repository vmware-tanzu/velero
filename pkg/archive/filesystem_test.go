/*
Copyright the Velero contributors.

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

package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestGetItemFilePath(t *testing.T) {
	res := GetItemFilePath("root", "resource", "", "item")
	assert.Equal(t, "root/resources/resource/cluster/item.json", res)

	res = GetItemFilePath("root", "resource", "namespace", "item")
	assert.Equal(t, "root/resources/resource/namespaces/namespace/item.json", res)

	res = GetItemFilePath("", "resource", "", "item")
	assert.Equal(t, "resources/resource/cluster/item.json", res)

	res = GetVersionedItemFilePath("root", "resource", "", "item", "")
	assert.Equal(t, "root/resources/resource/cluster/item.json", res)

	res = GetVersionedItemFilePath("root", "resource", "namespace", "item", "")
	assert.Equal(t, "root/resources/resource/namespaces/namespace/item.json", res)

	res = GetVersionedItemFilePath("root", "resource", "namespace", "item", "v1")
	assert.Equal(t, "root/resources/resource/v1/namespaces/namespace/item.json", res)

	res = GetVersionedItemFilePath("root", "resource", "", "item", "v1")
	assert.Equal(t, "root/resources/resource/v1/cluster/item.json", res)

	res = GetVersionedItemFilePath("", "resource", "", "item", "")
	assert.Equal(t, "resources/resource/cluster/item.json", res)
}

func TestGetScopeDir(t *testing.T) {
	res := GetScopeDir("")
	assert.Equal(t, velerov1api.ClusterScopedDir, res)

	res = GetScopeDir("test-namespace")
	assert.Equal(t, velerov1api.NamespaceScopedDir, res)
}

func TestUnmarshal(t *testing.T) {
	fs := test.NewFakeFileSystem()
	filePath := "pod.json"
	fileContent := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "example-pod"
		},
		"spec": {
			"containers": [{
				"name": "example-container",
				"image": "example-image"
			}]
		}
	}`
	out, err := fs.Create(filePath)
	require.NoError(t, err)

	_, err = out.Write([]byte(fileContent))
	require.NoError(t, err)

	_, err = Unmarshal(fs, filePath)
	require.NoError(t, err)
}
