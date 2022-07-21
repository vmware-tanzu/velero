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

package provider

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	repoconfig "github.com/vmware-tanzu/velero/pkg/repository/config"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/util/ownership"
)

type unifiedRepoProvider struct {
	credentialsFileStore credentials.FileStore
	workPath             string
	repoService          udmrepo.BackupRepoService
	log                  logrus.FieldLogger
}

// this func is assigned to a package-level variable so it can be
// replaced when unit-testing
var getAzureCredentials = repoconfig.GetAzureCredentials
var getS3Credentials = repoconfig.GetS3Credentials
var getGCPCredentials = repoconfig.GetGCPCredentials
var getS3BucketRegion = repoconfig.GetAWSBucketRegion
var getAzureStorageDomain = repoconfig.GetAzureStorageDomain

// NewUnifiedRepoProvider creates the service provider for Unified Repo
// workPath is the path for Unified Repo to store some local information
// workPath could be empty, if so, the default path will be used
func NewUnifiedRepoProvider(
	credentialFileStore credentials.FileStore,
	workPath string,
	log logrus.FieldLogger,
) (Provider, error) {
	repo := unifiedRepoProvider{
		credentialsFileStore: credentialFileStore,
		workPath:             workPath,
		log:                  log,
	}

	repo.repoService = createRepoService(log)

	log.Debug("Finished create unified repo service")

	return &repo, nil
}

func (urp *unifiedRepoProvider) InitRepo(ctx context.Context, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name": param.BackupLocation.Name,
		"BSL UID":  param.BackupLocation.UID,
	})

	log.Debug("Start to init repo")

	repoOption, err := urp.getRepoOption(param)
	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Init(ctx, repoOption, true)
	if err != nil {
		return errors.Wrap(err, "error to init backup repo")
	}

	log.Debug("Init repo complete")

	return nil
}

func (urp *unifiedRepoProvider) ConnectToRepo(ctx context.Context, param RepoParam) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PrepareRepo(ctx context.Context, param RepoParam) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PruneRepo(ctx context.Context, param RepoParam) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PruneRepoQuick(ctx context.Context, param RepoParam) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) EnsureUnlockRepo(ctx context.Context, param RepoParam) error {
	return nil
}

func (urp *unifiedRepoProvider) Forget(ctx context.Context, snapshotID string, param RepoParam) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) getRepoPassword(param RepoParam) (string, error) {
	///TODO: get repo password

	return "", nil
}

func (urp *unifiedRepoProvider) getRepoOption(param RepoParam) (udmrepo.RepoOptions, error) {
	repoOption := udmrepo.RepoOptions{
		StorageType:    getStorageType(param.BackupLocation),
		ConfigFilePath: getRepoConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		Ownership: udmrepo.OwnershipOptions{
			Username:   ownership.GetRepositoryOwner().Username,
			DomainName: ownership.GetRepositoryOwner().DomainName,
		},
		StorageOptions: make(map[string]string),
		GeneralOptions: make(map[string]string),
	}

	repoPassword, err := urp.getRepoPassword(param)
	if err != nil {
		return repoOption, errors.Wrap(err, "error to get repo password")
	}

	repoOption.RepoPassword = repoPassword

	storeVar, err := getStorageVariables(param.BackupLocation, param.SubDir)
	if err != nil {
		return repoOption, errors.Wrap(err, "error to get storage variables")
	}

	for k, v := range storeVar {
		repoOption.StorageOptions[k] = v
	}

	storeCred, err := getStorageCredentials(param.BackupLocation, urp.credentialsFileStore)
	if err != nil {
		return repoOption, errors.Wrap(err, "error to get repo credential env")
	}

	for k, v := range storeCred {
		repoOption.StorageOptions[k] = v
	}

	return repoOption, nil
}

func getStorageType(backupLocation *velerov1api.BackupStorageLocation) string {
	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)

	switch backendType {
	case repoconfig.AWSBackend:
		return udmrepo.StorageTypeS3
	case repoconfig.AzureBackend:
		return udmrepo.StorageTypeAzure
	case repoconfig.GCPBackend:
		return udmrepo.StorageTypeGcs
	case repoconfig.FSBackend:
		return udmrepo.StorageTypeFs
	default:
		return ""
	}
}

func getStorageCredentials(backupLocation *velerov1api.BackupStorageLocation, credentialsFileStore credentials.FileStore) (map[string]string, error) {
	result := make(map[string]string)
	var err error

	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)
	if !repoconfig.IsBackendTypeValid(backendType) {
		return map[string]string{}, errors.New("invalid storage provider")
	}

	config := backupLocation.Spec.Config
	if config == nil {
		config = map[string]string{}
	}

	if backupLocation.Spec.Credential != nil {
		config[repoconfig.CredentialsFileKey], err = credentialsFileStore.Path(backupLocation.Spec.Credential)
		if err != nil {
			return map[string]string{}, errors.Wrap(err, "error get credential file in bsl")
		}
	}

	switch backendType {
	case repoconfig.AWSBackend:
		credValue, err := getS3Credentials(config)
		if err != nil {
			return map[string]string{}, errors.Wrap(err, "error get s3 credentials")
		}
		result[udmrepo.StoreOptionS3KeyId] = credValue.AccessKeyID
		result[udmrepo.StoreOptionS3Provider] = credValue.ProviderName
		result[udmrepo.StoreOptionS3SecretKey] = credValue.SecretAccessKey
		result[udmrepo.StoreOptionS3Token] = credValue.SessionToken

	case repoconfig.AzureBackend:
		storageAccount, accountKey, err := getAzureCredentials(config)
		if err != nil {
			return map[string]string{}, errors.Wrap(err, "error get azure credentials")
		}
		result[udmrepo.StoreOptionAzureStorageAccount] = storageAccount
		result[udmrepo.StoreOptionAzureKey] = accountKey

	case repoconfig.GCPBackend:
		result[udmrepo.StoreOptionCredentialFile] = getGCPCredentials(config)
	}

	return result, nil
}

func getStorageVariables(backupLocation *velerov1api.BackupStorageLocation, repoName string) (map[string]string, error) {
	result := make(map[string]string)

	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)
	if !repoconfig.IsBackendTypeValid(backendType) {
		return map[string]string{}, errors.New("invalid storage provider")
	}

	config := backupLocation.Spec.Config
	if config == nil {
		config = map[string]string{}
	}

	bucket := strings.Trim(config["bucket"], "/")
	prefix := strings.Trim(config["prefix"], "/")
	if backupLocation.Spec.ObjectStorage != nil {
		bucket = strings.Trim(backupLocation.Spec.ObjectStorage.Bucket, "/")
		prefix = strings.Trim(backupLocation.Spec.ObjectStorage.Prefix, "/")
	}

	prefix = path.Join(prefix, udmrepo.StoreOptionPrefixName, repoName) + "/"

	region := config["region"]

	if backendType == repoconfig.AWSBackend {
		s3Url := config["s3Url"]

		var err error
		if s3Url == "" {
			region, err = getS3BucketRegion(bucket)
			if err != nil {
				return map[string]string{}, errors.Wrap(err, "error get s3 bucket region")
			}

			s3Url = fmt.Sprintf("s3-%s.amazonaws.com", region)
		}

		result[udmrepo.StoreOptionS3Endpoint] = strings.Trim(s3Url, "/")
		result[udmrepo.StoreOptionS3DisableTlsVerify] = config["insecureSkipTLSVerify"]
	} else if backendType == repoconfig.AzureBackend {
		result[udmrepo.StoreOptionAzureDomain] = getAzureStorageDomain(config)
	}

	result[udmrepo.StoreOptionOssBucket] = bucket
	result[udmrepo.StoreOptionPrefix] = prefix
	result[udmrepo.StoreOptionOssRegion] = strings.Trim(region, "/")
	result[udmrepo.StoreOptionFsPath] = config["fspath"]

	return result, nil
}

func getRepoConfigFile(workPath string, repoID string) string {
	///TODO: call udmrepo to get config file
	return ""
}

func createRepoService(log logrus.FieldLogger) udmrepo.BackupRepoService {
	///TODO: call udmrepo create repo service
	return nil
}
