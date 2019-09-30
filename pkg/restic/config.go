/*
Copyright 2018, 2019 the Velero contributors.

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
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/cloudprovider/aws"
	"github.com/vmware-tanzu/velero/pkg/persistence"
)

type BackendType string

const (
	AWSBackend   BackendType = "velero.io/aws"
	AzureBackend BackendType = "velero.io/azure"
	GCPBackend   BackendType = "velero.io/gcp"
)

// this func is assigned to a package-level variable so it can be
// replaced when unit-testing
var getAWSBucketRegion = aws.GetBucketRegion

// getRepoPrefix returns the prefix of the value of the --repo flag for
// restic commands, i.e. everything except the "/<repo-name>".
func getRepoPrefix(location *velerov1api.BackupStorageLocation) (string, error) {
	var bucket, prefix string

	if location.Spec.ObjectStorage != nil {
		layout := persistence.NewObjectStoreLayout(location.Spec.ObjectStorage.Prefix)

		bucket = location.Spec.ObjectStorage.Bucket
		prefix = layout.GetResticDir()
	}

	var provider = location.Spec.Provider
	if !strings.Contains(provider, "/") {
		provider = "velero.io/" + provider
	}

	if repoPrefix := location.Spec.Config["resticRepoPrefix"]; repoPrefix != "" {
		return repoPrefix, nil
	}

	switch BackendType(provider) {
	case AWSBackend:
		var url string
		switch {
		// non-AWS, S3-compatible object store
		case location.Spec.Config["s3Url"] != "":
			url = location.Spec.Config["s3Url"]
		default:
			region, err := getAWSBucketRegion(bucket)
			if err != nil {
				url = "s3.amazonaws.com"
				break
			}

			url = fmt.Sprintf("s3-%s.amazonaws.com", region)
		}

		return fmt.Sprintf("s3:%s/%s", strings.TrimSuffix(url, "/"), path.Join(bucket, prefix)), nil
	case AzureBackend:
		return fmt.Sprintf("azure:%s:/%s", bucket, prefix), nil
	case GCPBackend:
		return fmt.Sprintf("gs:%s:/%s", bucket, prefix), nil
	}

	return "", errors.New("restic repository prefix (resticRepoPrefix) not specified in backup storage location's config")
}

// GetRepoIdentifier returns the string to be used as the value of the --repo flag in
// restic commands for the given repository.
func GetRepoIdentifier(location *velerov1api.BackupStorageLocation, name string) (string, error) {
	prefix, err := getRepoPrefix(location)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", strings.TrimSuffix(prefix, "/"), name), nil
}
