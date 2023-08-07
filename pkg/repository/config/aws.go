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

//nolint:gosec
package config

import (
	"context"
	"os"

	goerr "errors"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

const (
	// AWS specific environment variable
	awsProfileEnvVar         = "AWS_PROFILE"
	awsProfileKey            = "profile"
	awsCredentialsFileEnvVar = "AWS_SHARED_CREDENTIALS_FILE"
)

// GetS3ResticEnvVars gets the environment variables that restic
// relies on (AWS_PROFILE) based on info in the provided object
// storage location config map.
func GetS3ResticEnvVars(config map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	if credentialsFile, ok := config[CredentialsFileKey]; ok {
		result[awsCredentialsFileEnvVar] = credentialsFile
	}

	if profile, ok := config[awsProfileKey]; ok {
		result[awsProfileEnvVar] = profile
	}

	return result, nil
}

// GetS3Credentials gets the S3 credential values according to the information
// of the provided config or the system's environment variables
func GetS3Credentials(config map[string]string) (*credentials.Value, error) {
	if len(os.Getenv("AWS_ROLE_ARN")) > 0 {
		return nil, nil
	}

	credentialsFile := config[CredentialsFileKey]
	if credentialsFile == "" {
		credentialsFile = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	}

	if credentialsFile == "" {
		return nil, errors.New("missing credential file")
	}

	creds := credentials.NewSharedCredentials(credentialsFile, config[awsProfileKey])
	credValue, err := creds.Get()
	if err != nil {
		return nil, err
	}

	return &credValue, nil
}

// GetAWSBucketRegion returns the AWS region that a bucket is in, or an error
// if the region cannot be determined.
func GetAWSBucketRegion(bucket string) (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", errors.WithStack(err)
	}

	var region string
	var requestErrs []error

	for _, partition := range endpoints.DefaultPartitions() {
		for regionHint := range partition.Regions() {
			region, err = s3manager.GetBucketRegion(context.Background(), sess, bucket, regionHint)
			if err != nil {
				requestErrs = append(requestErrs, errors.Wrapf(err, "error to get region with hint %s", regionHint))
			}

			// we only need to try a single region hint per partition, so break after the first
			break
		}

		if region != "" {
			return region, nil
		}
	}

	if requestErrs == nil {
		return "", errors.Errorf("unable to determine region by bucket %s", bucket)
	} else {
		return "", errors.Wrapf(goerr.Join(requestErrs...), "error to get region by bucket %s", bucket)
	}
}
