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
	"fmt"
	"strings"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider/aws"
)

type BackendType string

const (
	AWSBackend   BackendType = "aws"
	AzureBackend BackendType = "azure"
	GCPBackend   BackendType = "gcp"
)

// this func is assigned to a package-level variable so it can be
// replaced when unit-testing
var getAWSBucketRegion = aws.GetBucketRegion

// getRepoPrefix returns the prefix of the value of the --repo flag for
// restic commands, i.e. everything except the "/<repo-name>".
func getRepoPrefix(location *arkv1api.BackupStorageLocation) string {
	var (
		resticLocation       = location.Spec.Config[ResticLocationConfigKey]
		parts                = strings.SplitN(resticLocation, "/", 2)
		bucket, path, prefix string
	)

	if len(parts) >= 1 {
		bucket = parts[0]
	}
	if len(parts) >= 2 {
		path = parts[1]
	}

	switch BackendType(location.Spec.Provider) {
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

		return fmt.Sprintf("s3:%s/%s", url, resticLocation)
	case AzureBackend:
		prefix = "azure"
	case GCPBackend:
		prefix = "gs"
	}

	return fmt.Sprintf("%s:%s:/%s", prefix, bucket, path)
}

// GetRepoIdentifier returns the string to be used as the value of the --repo flag in
// restic commands for the given repository.
func GetRepoIdentifier(location *arkv1api.BackupStorageLocation, name string) string {
	prefix := getRepoPrefix(location)

	return fmt.Sprintf("%s/%s", strings.TrimSuffix(prefix, "/"), name)
}
