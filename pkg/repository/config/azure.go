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

package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	storagemgmt "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-06-01/storage"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

const (
	subscriptionIDEnvVar = "AZURE_SUBSCRIPTION_ID"
	cloudNameEnvVar      = "AZURE_CLOUD_NAME"

	resourceGroupConfigKey = "resourceGroup"

	storageAccountConfigKey          = "storageAccount"
	storageAccountKeyEnvVarConfigKey = "storageAccountKeyEnvVar"
	subscriptionIDConfigKey          = "subscriptionId"
	storageDomainConfigKey           = "storageDomain"
)

// getSubscriptionID gets the subscription ID from the 'config' map if it contains
// it, else from the AZURE_SUBSCRIPTION_ID environment variable.
func getSubscriptionID(config map[string]string) string {
	if subscriptionID := config[subscriptionIDConfigKey]; subscriptionID != "" {
		return subscriptionID
	}

	return os.Getenv(subscriptionIDEnvVar)
}

func getStorageAccountKey(config map[string]string) (string, error) {
	credentialsFile := selectCredentialsFile(config)

	if err := loadCredentialsIntoEnv(credentialsFile); err != nil {
		return "", err
	}

	// Get Azure cloud from AZURE_CLOUD_NAME, if it exists. If the env var does not
	// exist, parseAzureEnvironment will return azure.PublicCloud.
	env, err := parseAzureEnvironment(os.Getenv(cloudNameEnvVar))
	if err != nil {
		return "", errors.Wrap(err, "unable to parse azure cloud name environment variable")
	}

	// Get storage key from secret using key config[storageAccountKeyEnvVarConfigKey]. If the config does not
	// exist, continue obtaining it using API
	if secretKeyEnvVar := config[storageAccountKeyEnvVarConfigKey]; secretKeyEnvVar != "" {
		storageKey := os.Getenv(secretKeyEnvVar)
		if storageKey == "" {
			return "", errors.Errorf("no storage key secret with key %s found", secretKeyEnvVar)
		}

		return storageKey, nil
	}

	// get subscription ID from object store config or AZURE_SUBSCRIPTION_ID environment variable
	subscriptionID := getSubscriptionID(config)
	if subscriptionID == "" {
		return "", errors.New("azure subscription ID not found in object store's config or in environment variable")
	}

	// we need config["resourceGroup"], config["storageAccount"]
	if err := getRequiredValues(mapLookup(config), resourceGroupConfigKey, storageAccountConfigKey); err != nil {
		return "", errors.Wrap(err, "unable to get all required config values")
	}

	// get authorizer from environment in the following order:
	// 1. client credentials (AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET)
	// 2. client certificate (AZURE_CERTIFICATE_PATH, AZURE_CERTIFICATE_PASSWORD)
	// 3. username and password (AZURE_USERNAME, AZURE_PASSWORD)
	// 4. MSI (managed service identity)
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "error getting authorizer from environment")
	}

	// get storageAccountsClient
	storageAccountsClient := storagemgmt.NewAccountsClientWithBaseURI(env.ResourceManagerEndpoint, subscriptionID)
	storageAccountsClient.Authorizer = authorizer

	// get storage key
	res, err := storageAccountsClient.ListKeys(context.TODO(), config[resourceGroupConfigKey], config[storageAccountConfigKey], storagemgmt.Kerb)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if res.Keys == nil || len(*res.Keys) == 0 {
		return "", errors.New("No storage keys found")
	}

	var storageKey string
	for _, key := range *res.Keys {
		// The ListKeys call returns e.g. "FULL" but the storagemgmt.Full constant in the SDK is defined as "Full".
		if strings.EqualFold(string(key.Permissions), string(storagemgmt.Full)) {
			storageKey = *key.Value
			break
		}
	}

	if storageKey == "" {
		return "", errors.New("No storage key with Full permissions found")
	}

	return storageKey, nil
}

func mapLookup(data map[string]string) func(string) string {
	return func(key string) string {
		return data[key]
	}
}

// GetAzureResticEnvVars gets the environment variables that restic
// relies on (AZURE_ACCOUNT_NAME and AZURE_ACCOUNT_KEY) based
// on info in the provided object storage location config map.
func GetAzureResticEnvVars(config map[string]string) (map[string]string, error) {
	storageAccountKey, err := getStorageAccountKey(config)
	if err != nil {
		return nil, err
	}

	if err := getRequiredValues(mapLookup(config), storageAccountConfigKey); err != nil {
		return nil, errors.Wrap(err, "unable to get all required config values")
	}

	return map[string]string{
		"AZURE_ACCOUNT_NAME": config[storageAccountConfigKey],
		"AZURE_ACCOUNT_KEY":  storageAccountKey,
	}, nil
}

// credentialsFileFromEnv retrieves the Azure credentials file from the environment.
func credentialsFileFromEnv() string {
	return os.Getenv("AZURE_CREDENTIALS_FILE")
}

// selectCredentialsFile selects the Azure credentials file to use, retrieving it
// from the given config or falling back to retrieving it from the environment.
func selectCredentialsFile(config map[string]string) string {
	if credentialsFile, ok := config[CredentialsFileKey]; ok {
		return credentialsFile
	}

	return credentialsFileFromEnv()
}

// loadCredentialsIntoEnv loads the variables in the given credentials
// file into the current environment.
func loadCredentialsIntoEnv(credentialsFile string) error {
	if credentialsFile == "" {
		return nil
	}

	if err := godotenv.Overload(credentialsFile); err != nil {
		return errors.Wrapf(err, "error loading environment from credentials file (%s)", credentialsFile)
	}

	return nil
}

// ParseAzureEnvironment returns an azure.Environment for the given cloud
// name, or azure.PublicCloud if cloudName is empty.
func parseAzureEnvironment(cloudName string) (*azure.Environment, error) {
	if cloudName == "" {
		return &azure.PublicCloud, nil
	}

	env, err := azure.EnvironmentFromName(cloudName)
	return &env, errors.WithStack(err)
}

func getRequiredValues(getValue func(string) string, keys ...string) error {
	missing := []string{}
	results := map[string]string{}

	for _, key := range keys {
		if val := getValue(key); val == "" {
			missing = append(missing, key)
		} else {
			results[key] = val
		}
	}

	if len(missing) > 0 {
		return errors.Errorf("the following keys do not have values: %s", strings.Join(missing, ", "))
	}

	return nil
}

// GetAzureStorageDomain gets the Azure storage domain required by a Azure blob connection,
// if the provided credential file doesn't have the value, get it from system's environment variables
func GetAzureStorageDomain(config map[string]string) (string, error) {
	credentialsFile := selectCredentialsFile(config)

	if err := loadCredentialsIntoEnv(credentialsFile); err != nil {
		return "", err
	}

	return getStorageDomainFromCloudName(os.Getenv(cloudNameEnvVar))
}

func GetAzureCredentials(config map[string]string) (string, string, error) {
	storageAccountKey, err := getStorageAccountKey(config)
	if err != nil {
		return "", "", err
	}

	return config[storageAccountConfigKey], storageAccountKey, nil
}

func getStorageDomainFromCloudName(cloudName string) (string, error) {
	env, err := parseAzureEnvironment(cloudName)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse azure env from cloud name %s", cloudName)
	}

	return fmt.Sprintf("blob.%s", env.StorageEndpointSuffix), nil
}
