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
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	reposervice "github.com/vmware-tanzu/velero/pkg/repository/udmrepo/service"
)

type unifiedRepoProvider struct {
	credentialGetter credentials.CredentialGetter
	workPath         string
	repoService      udmrepo.BackupRepoService
	log              logrus.FieldLogger
}

// this func is assigned to a package-level variable so it can be
// replaced when unit-testing
var getAzureCredentials = repoconfig.GetAzureCredentials
var getS3Credentials = repoconfig.GetS3Credentials
var getGCPCredentials = repoconfig.GetGCPCredentials
var getS3BucketRegion = repoconfig.GetAWSBucketRegion
var getAzureStorageDomain = repoconfig.GetAzureStorageDomain

type localFuncTable struct {
	getStorageVariables   func(*velerov1api.BackupStorageLocation, string) (map[string]string, error)
	getStorageCredentials func(*velerov1api.BackupStorageLocation, credentials.FileStore) (map[string]string, error)
}

var funcTable = localFuncTable{
	getStorageVariables:   getStorageVariables,
	getStorageCredentials: getStorageCredentials,
}

const (
	repoOpDescFullMaintain  = "full maintenance"
	repoOpDescQuickMaintain = "quick maintenance"
	repoOpDescForget        = "forget"
)

// NewUnifiedRepoProvider creates the service provider for Unified Repo
func NewUnifiedRepoProvider(
	credentialGetter credentials.CredentialGetter,
	log logrus.FieldLogger,
) (Provider, error) {
	repo := unifiedRepoProvider{
		credentialGetter: credentialGetter,
		log:              log,
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

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithStoreOptions(urp, param),
		udmrepo.WithDescription(repoOpDescFullMaintain),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Init(ctx, *repoOption, true)
	if err != nil {
		return errors.Wrap(err, "error to init backup repo")
	}

	log.Debug("Init repo complete")

	return nil
}

func (urp *unifiedRepoProvider) ConnectToRepo(ctx context.Context, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name": param.BackupLocation.Name,
		"BSL UID":  param.BackupLocation.UID,
	})

	log.Debug("Start to connect repo")

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithStoreOptions(urp, param),
		udmrepo.WithDescription(repoOpDescFullMaintain),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Init(ctx, *repoOption, false)
	if err != nil {
		return errors.Wrap(err, "error to connect backup repo")
	}

	log.Debug("Connect repo complete")

	return nil
}

func (urp *unifiedRepoProvider) PrepareRepo(ctx context.Context, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name": param.BackupLocation.Name,
		"BSL UID":  param.BackupLocation.UID,
	})

	log.Debug("Start to prepare repo")

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithStoreOptions(urp, param),
		udmrepo.WithDescription(repoOpDescFullMaintain),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Init(ctx, *repoOption, false)
	if err == nil {
		log.Debug("Repo has already been initialized remotely")
		return nil
	}

	err = urp.repoService.Init(ctx, *repoOption, true)
	if err != nil {
		return errors.Wrap(err, "error to init backup repo")
	}

	log.Debug("Prepare repo complete")

	return nil
}

func (urp *unifiedRepoProvider) PruneRepo(ctx context.Context, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name": param.BackupLocation.Name,
		"BSL UID":  param.BackupLocation.UID,
	})

	log.Debug("Start to prune repo")

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithGenOptions(map[string]string{udmrepo.GenOptionMaintainMode: udmrepo.GenOptionMaintainFull}),
		udmrepo.WithDescription(repoOpDescFullMaintain),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Maintain(ctx, *repoOption)
	if err != nil {
		return errors.Wrap(err, "error to prune backup repo")
	}

	log.Debug("Prune repo complete")

	return nil
}

func (urp *unifiedRepoProvider) PruneRepoQuick(ctx context.Context, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name": param.BackupLocation.Name,
		"BSL UID":  param.BackupLocation.UID,
	})

	log.Debug("Start to prune repo quick")

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithGenOptions(map[string]string{udmrepo.GenOptionMaintainMode: udmrepo.GenOptionMaintainQuick}),
		udmrepo.WithDescription(repoOpDescQuickMaintain),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	err = urp.repoService.Maintain(ctx, *repoOption)
	if err != nil {
		return errors.Wrap(err, "error to prune backup repo quick")
	}

	log.Debug("Prune repo quick complete")

	return nil
}

func (urp *unifiedRepoProvider) EnsureUnlockRepo(ctx context.Context, param RepoParam) error {
	return nil
}

func (urp *unifiedRepoProvider) Forget(ctx context.Context, snapshotID string, param RepoParam) error {
	log := urp.log.WithFields(logrus.Fields{
		"BSL name":   param.BackupLocation.Name,
		"BSL UID":    param.BackupLocation.UID,
		"snapshotID": snapshotID,
	})

	log.Debug("Start to forget snapshot")

	repoOption, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(urp, param),
		udmrepo.WithConfigFile(urp.workPath, string(param.BackupLocation.UID)),
		udmrepo.WithDescription(repoOpDescForget),
	)

	if err != nil {
		return errors.Wrap(err, "error to get repo options")
	}

	bkRepo, err := urp.repoService.Open(ctx, *repoOption)
	if err != nil {
		return errors.Wrap(err, "error to open backup repo")
	}

	defer func() {
		c := bkRepo.Close(ctx)
		if c != nil {
			log.WithError(c).Error("Failed to close repo")
		}
	}()

	err = bkRepo.DeleteManifest(ctx, udmrepo.ID(snapshotID))
	if err != nil {
		return errors.Wrap(err, "error to delete manifest")
	}

	log.Debug("Forget snapshot complete")

	return nil
}

func (urp *unifiedRepoProvider) GetPassword(param interface{}) (string, error) {
	repoParam, ok := param.(RepoParam)
	if !ok {
		return "", errors.New("invalid parameter")
	}

	repoPassword, err := getRepoPassword(urp.credentialGetter.FromSecret, repoParam)
	if err != nil {
		return "", errors.Wrap(err, "error to get repo password")
	}

	return repoPassword, nil
}

func (urp *unifiedRepoProvider) GetStoreType(param interface{}) (string, error) {
	repoParam, ok := param.(RepoParam)
	if !ok {
		return "", errors.New("invalid parameter")
	}

	return getStorageType(repoParam.BackupLocation), nil
}

func (urp *unifiedRepoProvider) GetStoreOptions(param interface{}) (map[string]string, error) {
	repoParam, ok := param.(RepoParam)
	if !ok {
		return map[string]string{}, errors.New("invalid parameter")
	}

	storeVar, err := funcTable.getStorageVariables(repoParam.BackupLocation, repoParam.BackupRepo.Spec.VolumeNamespace)
	if err != nil {
		return map[string]string{}, errors.Wrap(err, "error to get storage variables")
	}

	storeCred, err := funcTable.getStorageCredentials(repoParam.BackupLocation, urp.credentialGetter.FromFile)
	if err != nil {
		return map[string]string{}, errors.Wrap(err, "error to get repo credentials")
	}

	storeOptions := make(map[string]string)
	for k, v := range storeVar {
		storeOptions[k] = v
	}

	for k, v := range storeCred {
		storeOptions[k] = v
	}

	return storeOptions, nil
}

func getRepoPassword(secretStore credentials.SecretStore, param RepoParam) (string, error) {
	if secretStore == nil {
		return "", errors.New("invalid credentials interface")
	}

	rawPass, err := secretStore.Get(repokey.RepoKeySelector())
	if err != nil {
		return "", errors.Wrap(err, "error to get password")
	}

	return strings.TrimSpace(rawPass), nil
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

	if credentialsFileStore == nil {
		return map[string]string{}, errors.New("invalid credentials interface")
	}

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

func createRepoService(log logrus.FieldLogger) udmrepo.BackupRepoService {
	return reposervice.Create(log)
}
