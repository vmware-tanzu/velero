package repoprovider

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/repoconfig"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/util/ownership"
)

type unifiedRepoProvider struct {
	ctx                  context.Context
	credentialsFileStore credentials.FileStore
	backupLocation       *velerov1api.BackupStorageLocation
	repoService          udmrepo.BackupRepoService
	repoPassword         string
	repoName             string
	configPath           string
	log                  *logrus.Logger
}

func NewUnifiedRepoProvider(
	ctx context.Context,
	credentialFileStore credentials.FileStore,
	backupLocation *velerov1api.BackupStorageLocation,
	configPath string,
	repoName string,
	log *logrus.Logger,
) (RepositoryProvider, error) {
	repo := unifiedRepoProvider{
		ctx:                  ctx,
		credentialsFileStore: credentialFileStore,
		backupLocation:       backupLocation,
		configPath:           configPath,
		repoName:             repoName,
		log:                  log,
	}

	repo.repoService = createRepoService(log)

	log.Debug("Finished create unified repo service")

	return &repo, nil
}

func (urp *unifiedRepoProvider) InitRepo(ctx context.Context, backupLocation string) error {
	log := urp.log.WithFields(logrus.Fields{
		"backupLocation": backupLocation,
	})

	log.Debug("Start to init repo")

	err := urp.ensureRepoPassword()
	if err != nil {
		log.WithError(err).Error("Failed to ensure repo password")
		return err
	}

	repoOption, err := urp.getRepoOption()
	if err != nil {
		log.WithError(err).Error("Failed to get repo options")
		return err
	}

	err = urp.repoService.Init(ctx, repoOption, true)
	if err != nil {
		log.WithError(err).Error("Failed to init backup repo")
	}

	return err
}

func (urp *unifiedRepoProvider) ConnectToRepo(ctx context.Context, backupLocation string) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PrepareRepo(ctx context.Context, backupLocation string) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PruneRepo(ctx context.Context, backupLocation string) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) PruneRepoQuick(ctx context.Context, backupLocation string) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) EnsureUnlockRepo(ctx context.Context, backupLocation string) error {
	return nil
}

func (urp *unifiedRepoProvider) Forget(ctx context.Context, snapshotID string, backupLocation string) error {
	///TODO
	return nil
}

func (urp *unifiedRepoProvider) ensureRepoPassword() error {
	if urp.repoPassword != "" {
		return nil
	}

	///TODO: get repo password

	return nil
}

func (urp *unifiedRepoProvider) getRepoOption() (udmrepo.RepoOptions, error) {
	log := urp.log

	repoOption := udmrepo.RepoOptions{
		StorageType:    getStorageType(urp.backupLocation),
		RepoPassword:   urp.repoPassword,
		ConfigFilePath: urp.configPath,
		StorageOptions: make(map[string]string),
		GeneralOptions: map[string]string{
			udmrepo.UNFIED_REPO_GEN_OPTION_OWNER_DOMAIN: ownership.GetBackupOwner().DomainName,
			udmrepo.UNIFIED_REPO_GEN_OPTION_OWNER_NAME:  ownership.GetBackupOwner().Username,
		},
	}

	storeVar, err := getStorageVariables(urp.backupLocation, urp.repoName)
	if err != nil {
		log.WithError(err).Error("Failed to emend storage variables")
		return repoOption, err
	}

	for k, v := range storeVar {
		repoOption.StorageOptions[k] = v
	}

	storeCred, err := getStorageCredentials(urp.backupLocation, urp.credentialsFileStore, urp.log)
	if err != nil {
		log.WithError(err).Error("Failed to get repo credential env")
		return repoOption, err
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
		return udmrepo.UNIFIED_REPO_OPTION_STORAGE_TYPE_S3
	case repoconfig.AzureBackend:
		return udmrepo.UNIFIED_REPO_OPTION_STORAGE_TYPE_AZURE
	case repoconfig.GCPBackend:
		return udmrepo.UNIFIED_REPO_OPTION_STORAGE_TYPE_GCS
	case repoconfig.FSBackend:
		return udmrepo.UNIFIED_REPO_OPTION_STORAGE_TYPE_FS
	default:
		return ""
	}
}

func getStorageCredentials(backupLocation *velerov1api.BackupStorageLocation, credentialsFileStore credentials.FileStore, log *logrus.Logger) (map[string]string, error) {
	result := make(map[string]string)
	var err error
	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)

	config := backupLocation.Spec.Config
	if config == nil {
		config = map[string]string{}
	}

	if backupLocation.Spec.Credential != nil {
		config[repoconfig.CredentialsFileKey], err = credentialsFileStore.Path(backupLocation.Spec.Credential)
		if err != nil {
			log.WithError(err).Error("Failed to get credential file in BSL")
			return map[string]string{}, err
		}
	}

	switch backendType {
	case repoconfig.AWSBackend:
		credValue, err := repoconfig.GetS3Credentials(config)
		if err != nil {
			log.WithError(err).Error("Failed to get S3 credentials")
			return map[string]string{}, err
		}
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_KEY_ID] = credValue.AccessKeyID
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_PROVIDER] = credValue.ProviderName
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_SECRET_KEY] = credValue.SecretAccessKey
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_TOKEN] = credValue.SessionToken

	case repoconfig.AzureBackend:
		storageAccount, accountKey, err := repoconfig.GetAzureCredentials(config)
		if err != nil {
			log.WithError(err).Error("Failed to get Azure credentials")
			return map[string]string{}, err
		}
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_AZ_STORAGE_ACCOUNT] = storageAccount
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_AZ_KEY] = accountKey

	case repoconfig.GCPBackend:
		result[udmrepo.UNIFIED_REPO_STORE_OPTION_CRED_FILE] = repoconfig.GetGCPCredentials(config)
	}

	return result, nil
}

func getStorageVariables(backupLocation *velerov1api.BackupStorageLocation, repoName string) (map[string]string, error) {
	result := make(map[string]string)
	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)

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

	prefix = path.Join(prefix, udmrepo.UNIFIED_REPO_STORE_OPTION_PREFIX_NAME, repoName) + "/"

	s3Url := config["s3Url"]
	region := config["region"]
	var err error

	if backendType == repoconfig.AWSBackend {
		if s3Url == "" && region == "" {
			region, err = repoconfig.GetAWSBucketRegion(bucket)
			if err != nil {
				return map[string]string{}, err
			}

			s3Url = fmt.Sprintf("s3-%s.amazonaws.com", region)
		}
	}

	result[udmrepo.UNIFIED_REPO_STORE_OPTION_OSS_BUCKET] = bucket
	result[udmrepo.UNIFIED_REPO_STORE_OPTION_PREFIX] = prefix
	result[udmrepo.UNIFIED_REPO_STORE_OPTION_OSS_REGION] = strings.Trim(region, "/")

	result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_ENDPOINT] = strings.Trim(s3Url, "/")
	result[udmrepo.UNIFIED_REPO_STORE_OPTION_S3_DISABLE_TLS_VERIFY] = config["insecureSkipTLSVerify"]

	result[udmrepo.UNIFIED_REPO_STORE_OPTION_AZ_DOMAIN] = repoconfig.GetAzureStorageDomain(config)

	result[udmrepo.UNIFIED_REPO_STORE_OPTION_FS_PATH] = config["fspath"]

	return result, nil
}

func createRepoService(log *logrus.Logger) udmrepo.BackupRepoService {
	///TODO: add kopia_lib implementation
	return nil
}
