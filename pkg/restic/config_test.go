/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
)

func TestGetRepoIdentifier(t *testing.T) {
	// if getAWSBucketRegion returns an error, use default "s3.amazonaws.com/..." URL
	getAWSBucketRegion = func(string) (string, error) {
		return "", errors.New("no region found")
	}

	backupLocation := &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "aws",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket",
					Prefix: "prefix",
				},
			},
		},
	}
	assert.Equal(t, "s3:s3.amazonaws.com/bucket/prefix/restic/repo-1", GetRepoIdentifier(backupLocation, "repo-1"))

	// stub implementation of getAWSBucketRegion
	getAWSBucketRegion = func(string) (string, error) {
		return "us-west-2", nil
	}

	backupLocation = &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "aws",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket",
				},
			},
		},
	}
	assert.Equal(t, "s3:s3-us-west-2.amazonaws.com/bucket/restic/repo-1", GetRepoIdentifier(backupLocation, "repo-1"))

	backupLocation = &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "aws",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket",
					Prefix: "prefix",
				},
			},
		},
	}
	assert.Equal(t, "s3:s3-us-west-2.amazonaws.com/bucket/prefix/restic/repo-1", GetRepoIdentifier(backupLocation, "repo-1"))

	backupLocation = &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "aws",
			Config: map[string]string{
				"s3Url": "alternate-url",
			},
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket",
					Prefix: "prefix",
				},
			},
		},
	}
	assert.Equal(t, "s3:alternate-url/bucket/prefix/restic/repo-1", GetRepoIdentifier(backupLocation, "repo-1"))

	backupLocation = &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "azure",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket",
					Prefix: "prefix",
				},
			},
		},
	}
	assert.Equal(t, "azure:bucket:/prefix/restic/repo-1", GetRepoIdentifier(backupLocation, "repo-1"))

	backupLocation = &velerov1api.BackupStorageLocation{
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "gcp",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket-2",
					Prefix: "prefix-2",
				},
			},
		},
	}
	assert.Equal(t, "gs:bucket-2:/prefix-2/restic/repo-2", GetRepoIdentifier(backupLocation, "repo-2"))
}
