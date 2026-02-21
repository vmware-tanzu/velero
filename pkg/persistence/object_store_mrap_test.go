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

package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestNewObjectBackupStoreGetter_MRAP(t *testing.T) {
	tests := []struct {
		name              string
		location          *velerov1api.BackupStorageLocation
		objectStoreGetter objectStoreGetter
		wantBucket        string
		wantErr           string
	}{
		{
			name:     "when the Bucket field is an MRAP ARN, it should be valid",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap").Result(),
			objectStoreGetter: objectStoreGetter{
				"provider-1": newInMemoryObjectStore("arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap"),
			},
			wantBucket: "arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap",
		},
		{
			name:     "when the Bucket field is an MRAP ARN with trailing slash, it should be valid and trimmed",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap/").Result(),
			objectStoreGetter: objectStoreGetter{
				"provider-1": newInMemoryObjectStore("arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap"),
			},
			wantBucket: "arn:aws:s3::123456789012:accesspoint/abcdef0123456.mrap",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getter := NewObjectBackupStoreGetter(velerotest.NewFakeCredentialsFileStore("", nil))
			res, err := getter.Get(tc.location, tc.objectStoreGetter, velerotest.NewLogger())
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)

				store, ok := res.(*objectBackupStore)
				require.True(t, ok)

				assert.Equal(t, tc.wantBucket, store.bucket)
			}
		})
	}
}
