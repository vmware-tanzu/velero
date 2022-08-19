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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"

	"github.com/kopia/kopia/repo/blob/azure"
	"github.com/kopia/kopia/repo/blob/throttling"
)

func TestAzureSetup(t *testing.T) {
	testCases := []struct {
		name        string
		flags       map[string]string
		expected    azure.Options
		expectedErr string
	}{
		{
			name:        "must have bucket name",
			flags:       map[string]string{},
			expectedErr: "key " + udmrepo.StoreOptionOssBucket + " not found",
		},
		{
			name: "must have storage account",
			flags: map[string]string{
				udmrepo.StoreOptionOssBucket: "fake-bucket",
			},
			expected: azure.Options{
				Container: "fake-bucket",
			},
			expectedErr: "key " + udmrepo.StoreOptionAzureStorageAccount + " not found",
		},
		{
			name: "must have secret key",
			flags: map[string]string{
				udmrepo.StoreOptionOssBucket:           "fake-bucket",
				udmrepo.StoreOptionAzureStorageAccount: "fake-account",
			},
			expected: azure.Options{
				Container:      "fake-bucket",
				StorageAccount: "fake-account",
			},
			expectedErr: "key " + udmrepo.StoreOptionAzureKey + " not found",
		},
		{
			name: "with limits",
			flags: map[string]string{
				udmrepo.StoreOptionOssBucket:           "fake-bucket",
				udmrepo.StoreOptionAzureStorageAccount: "fake-account",
				udmrepo.StoreOptionAzureKey:            "fake-key",
				udmrepo.ThrottleOptionReadOps:          "100",
				udmrepo.ThrottleOptionUploadBytes:      "200",
			},
			expected: azure.Options{
				Container:      "fake-bucket",
				StorageAccount: "fake-account",
				StorageKey:     "fake-key",
				Limits: throttling.Limits{
					ReadsPerSecond:       100,
					UploadBytesPerSecond: 200,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			azFlags := AzureBackend{}

			err := azFlags.Setup(context.Background(), tc.flags)

			require.Equal(t, tc.expected, azFlags.options)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
