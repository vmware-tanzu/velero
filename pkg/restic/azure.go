/*
Copyright 2017, 2019, 2020 the Velero contributors.

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
	"context"
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
)

// getSubscriptionID gets the subscription ID from the 'config' map if it contains
// it, else from the AZURE_SUBSCRIPTION_ID environment variable.
func getSubscriptionID(config map[string]string) string {
	if subscriptionID := config[subscriptionIDConfigKey]; subscriptionID != "" {
		return subscriptionID
	}

	return os.Getenv(subscriptionIDEnvVar)
}

func getStorageAccountKey(config map[string]string) (string, *azure.Environment, error) {
	// load environment vars from $AZURE_CREDENTIALS_FILE, if it exists
	if err := loadEnv(); err != nil {
		return "", nil, err
	}

	// Get Azure cloud from AZURE_CLOUD_NAME, if it exists. If the env var does not
	// exist, parseAzureEnvironment will return azure.PublicCloud.
	env, err := parseAzureEnvironment(os.Getenv(cloudNameEnvVar))
	if err != nil {
		return "", nil, errors.Wrap(err, "unable to parse azure cloud name environment variable")
	}

	// Get storage key from secret using key config[storageAccountKeyEnvVarConfigKey]. If the config does not
	// exist, continue obtaining it using API
	if secretKeyEnvVar := config[storageAccountKeyEnvVarConfigKey]; secretKeyEnvVar != "" {
		storageKey := os.Getenv(secretKeyEnvVar)
		if storageKey == "" {
			return "", env, errors.Errorf("no storage key secret with key %s found", secretKeyEnvVar)
		}

		return storageKey, env, nil
	}

	// get subscription ID from object store config or AZURE_SUBSCRIPTION_ID environment variable
	subscriptionID := getSubscriptionID(config)
	if subscriptionID == "" {
		return "", nil, errors.New("azure subscription ID not found in object store's config or in environment variable")
	}

	// we need config["resourceGroup"], config["storageAccount"]
	if _, err := getRequiredValues(mapLookup(config), resourceGroupConfigKey, storageAccountConfigKey); err != nil {
		return "", env, errors.Wrap(err, "unable to get all required config values")
	}

	// get authorizer from environment in the following order:
	// 1. client credentials (AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET)
	// 2. client certificate (AZURE_CERTIFICATE_PATH, AZURE_CERTIFICATE_PASSWORD)
	// 3. username and password (AZURE_USERNAME, AZURE_PASSWORD)
	// 4. MSI (managed service identity)
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return "", nil, errors.Wrap(err, "error getting authorizer from environment")
	}

	// get storageAccountsClient
	storageAccountsClient := storagemgmt.NewAccountsClientWithBaseURI(env.ResourceManagerEndpoint, subscriptionID)
	storageAccountsClient.Authorizer = authorizer

	// get storage key
	res, err := storageAccountsClient.ListKeys(context.TODO(), config[resourceGroupConfigKey], config[storageAccountConfigKey], storagemgmt.Kerb)
	if err != nil {
		return "", env, errors.WithStack(err)
	}
	if res.Keys == nil || len(*res.Keys) == 0 {
		return "", env, errors.New("No storage keys found")
	}

	var storageKey string
	for _, key := range *res.Keys {
		// uppercase both strings for comparison because the ListKeys call returns e.g. "FULL" but
		// the storagemgmt.Full constant in the SDK is defined as "Full".
		if strings.ToUpper(string(key.Permissions)) == strings.ToUpper(string(storagemgmt.Full)) {
			storageKey = *key.Value
			break
		}
	}

	if storageKey == "" {
		return "", env, errors.New("No storage key with Full permissions found")
	}

	return storageKey, env, nil
}

func mapLookup(data map[string]string) func(string) string {
	return func(key string) string {
		return data[key]
	}
}

// getAzureResticEnvVars gets the environment variables that restic
// relies on (AZURE_ACCOUNT_NAME and AZURE_ACCOUNT_KEY) based
// on info in the provided object storage location config map.
func getAzureResticEnvVars(config map[string]string) (map[string]string, error) {
	storageAccountKey, _, err := getStorageAccountKey(config)
	if err != nil {
		return nil, err
	}

	if _, err := getRequiredValues(mapLookup(config), storageAccountConfigKey); err != nil {
		return nil, errors.Wrap(err, "unable to get all required config values")
	}

	return map[string]string{
		"AZURE_ACCOUNT_NAME": config[storageAccountConfigKey],
		"AZURE_ACCOUNT_KEY":  storageAccountKey,
	}, nil
}

func loadEnv() error {
	envFile := os.Getenv("AZURE_CREDENTIALS_FILE")
	if envFile == "" {
		return nil
	}

	if err := godotenv.Overload(envFile); err != nil {
		return errors.Wrapf(err, "error loading environment from AZURE_CREDENTIALS_FILE (%s)", envFile)
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

func getRequiredValues(getValue func(string) string, keys ...string) (map[string]string, error) {
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
		return nil, errors.Errorf("the following keys do not have values: %s", strings.Join(missing, ", "))
	}

	return results, nil
}
