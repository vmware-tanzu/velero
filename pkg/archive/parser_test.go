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

package archive

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		dir     string
		wantErr error
		want    map[string]*ResourceItems
	}{
		{
			name:    "when there is no top-level resources directory, an error is returned",
			dir:     "root-dir",
			wantErr: errors.New("directory \"resources\" does not exist"),
		},
		{
			name:  "when there are no directories under the resources directory, an empty map is returned",
			dir:   "root-dir",
			files: []string{"root-dir/resources/"},
			want:  map[string]*ResourceItems{},
		},
		{
			name: "a mix of cluster-scoped and namespaced items across multiple resources are correctly returned",
			dir:  "root-dir",
			files: []string{
				"root-dir/resources/widgets.foo/cluster/item-1.json",
				"root-dir/resources/widgets.foo/cluster/item-2.json",
				"root-dir/resources/widgets.foo/namespaces/ns-1/item-1.json",
				"root-dir/resources/widgets.foo/namespaces/ns-1/item-2.json",
				"root-dir/resources/widgets.foo/namespaces/ns-2/item-1.json",
				"root-dir/resources/widgets.foo/namespaces/ns-2/item-2.json",

				"root-dir/resources/dongles.foo/cluster/item-3.json",
				"root-dir/resources/dongles.foo/cluster/item-4.json",

				"root-dir/resources/dongles.bar/namespaces/ns-3/item-3.json",
				"root-dir/resources/dongles.bar/namespaces/ns-3/item-4.json",
				"root-dir/resources/dongles.bar/namespaces/ns-4/item-5.json",
				"root-dir/resources/dongles.bar/namespaces/ns-4/item-6.json",
			},
			want: map[string]*ResourceItems{
				"widgets.foo": {
					GroupResource: "widgets.foo",
					ItemsByNamespace: map[string][]string{
						"":     {"item-1", "item-2"},
						"ns-1": {"item-1", "item-2"},
						"ns-2": {"item-1", "item-2"},
					},
				},
				"dongles.foo": {
					GroupResource: "dongles.foo",
					ItemsByNamespace: map[string][]string{
						"": {"item-3", "item-4"},
					},
				},
				"dongles.bar": {
					GroupResource: "dongles.bar",
					ItemsByNamespace: map[string][]string{
						"ns-3": {"item-3", "item-4"},
						"ns-4": {"item-5", "item-6"},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				log: test.NewLogger(),
				fs:  test.NewFakeFileSystem(),
			}

			for _, file := range tc.files {
				require.NoError(t, p.fs.MkdirAll(file, 0755))

				if !strings.HasSuffix(file, "/") {
					res, err := p.fs.Create(file)
					require.NoError(t, err)
					require.NoError(t, res.Close())
				}
			}

			res, err := p.Parse(tc.dir)
			if tc.wantErr != nil {
				assert.Equal(t, err.Error(), tc.wantErr.Error())
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.want, res)
			}
		})
	}
}
