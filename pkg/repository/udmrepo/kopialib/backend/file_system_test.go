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

package backend

import (
	"context"
	"testing"

	"github.com/kopia/kopia/repo/blob/filesystem"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

func TestFSSetup(t *testing.T) {
	testCases := []struct {
		name            string
		flags           map[string]string
		expectedOptions filesystem.Options
		expectedErr     string
	}{
		{
			name:        "must have fs path",
			flags:       map[string]string{},
			expectedErr: "key " + udmrepo.StoreOptionFsPath + " not found",
		},
		{
			name: "with fs path only",
			flags: map[string]string{
				udmrepo.StoreOptionFsPath: "fake/path",
			},
			expectedOptions: filesystem.Options{
				Path:          "fake/path",
				FileMode:      0o600,
				DirectoryMode: 0o700,
			},
		},
		{
			name: "with prefix",
			flags: map[string]string{
				udmrepo.StoreOptionFsPath: "fake/path",
				udmrepo.StoreOptionPrefix: "fake-prefix",
			},
			expectedOptions: filesystem.Options{
				Path:          "fake/path/fake-prefix",
				FileMode:      0o600,
				DirectoryMode: 0o700,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fsFlags := FsBackend{}

			err := fsFlags.Setup(context.Background(), tc.flags)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedOptions, fsFlags.options)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
