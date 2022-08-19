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

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

func TestS3Setup(t *testing.T) {
	testCases := []struct {
		name        string
		flags       map[string]string
		expectedErr string
	}{
		{
			name:        "must have bucket name",
			flags:       map[string]string{},
			expectedErr: "key " + udmrepo.StoreOptionOssBucket + " not found",
		},
		{
			name: "must have access key Id",
			flags: map[string]string{
				udmrepo.StoreOptionOssBucket: "fake-bucket",
			},
			expectedErr: "key " + udmrepo.StoreOptionS3KeyId + " not found",
		},
		{
			name: "must have access key",
			flags: map[string]string{
				udmrepo.StoreOptionOssBucket: "fake-bucket",
				udmrepo.StoreOptionS3KeyId:   "fake-key-id",
			},
			expectedErr: "key " + udmrepo.StoreOptionS3SecretKey + " not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s3Flags := S3Backend{}

			err := s3Flags.Setup(context.Background(), tc.flags)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
