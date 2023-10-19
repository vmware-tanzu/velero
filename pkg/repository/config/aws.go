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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
)

const (
	// AWS specific environment variable
	awsProfileEnvVar         = "AWS_PROFILE"
	awsRoleEnvVar            = "AWS_ROLE_ARN"
	awsKeyIDEnvVar           = "AWS_ACCESS_KEY_ID"
	awsSecretKeyEnvVar       = "AWS_SECRET_ACCESS_KEY"
	awsSessTokenEnvVar       = "AWS_SESSION_TOKEN"
	awsProfileKey            = "profile"
	awsCredentialsFileEnvVar = "AWS_SHARED_CREDENTIALS_FILE"
	awsConfigFileEnvVar      = "AWS_CONFIG_FILE"
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

	// GetS3ResticEnvVars reads the AWS config, from files and envs
	// if needed assumes the role and returns the session credentials
	// setting these variables emulates what would happen for example when using kube2iam
	if creds, err := GetS3Credentials(config); err == nil && creds != nil {
		result[awsKeyIDEnvVar] = creds.AccessKeyID
		result[awsSecretKeyEnvVar] = creds.SecretAccessKey
		result[awsSessTokenEnvVar] = creds.SessionToken
		result[awsCredentialsFileEnvVar] = ""
		result[awsProfileEnvVar] = ""
		result[awsConfigFileEnvVar] = ""
	}

	return result, nil
}

// GetS3Credentials gets the S3 credential values according to the information
// of the provided config or the system's environment variables
func GetS3Credentials(config map[string]string) (*aws.Credentials, error) {
	if os.Getenv(awsRoleEnvVar) != "" {
		return nil, nil
	}

	var opts []func(*awsconfig.LoadOptions) error
	credentialsFile := config[CredentialsFileKey]
	if credentialsFile == "" {
		credentialsFile = os.Getenv(awsCredentialsFileEnvVar)
	}
	if credentialsFile != "" {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles([]string{credentialsFile}),
			// To support the existing use case where config file is passed
			// as credentials of a BSL
			awsconfig.WithSharedConfigFiles([]string{credentialsFile}))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	creds, err := cfg.Credentials.Retrieve(context.Background())

	return &creds, err
}

// GetAWSBucketRegion returns the AWS region that a bucket is in, or an error
// if the region cannot be determined.
func GetAWSBucketRegion(bucket string) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", errors.WithStack(err)
	}
	client := s3.NewFromConfig(cfg)
	region, err := s3manager.GetBucketRegion(context.Background(), client, bucket)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if region == "" {
		return "", errors.New("unable to determine bucket's region")
	}
	return region, nil
}
