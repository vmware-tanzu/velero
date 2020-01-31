/*
Copyright 2017, 2019 the Velero contributors.

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

	storagemgmt "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2018-02-01/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

const (
	tenantIDEnvVar       = "AZURE_TENANT_ID"
	subscriptionIDEnvVar = "AZURE_SUBSCRIPTION_ID"
	clientIDEnvVar       = "AZURE_CLIENT_ID"
	clientSecretEnvVar   = "AZURE_CLIENT_SECRET"
	cloudNameEnvVar      = "AZURE_CLOUD_NAME"

	resourceGroupConfigKey = "resourceGroup"

	storageAccountConfigKey = "storageAccount"
	subscriptionIdConfigKey = "subscriptionId"
)

func getStorageAccountKey(config map[string]string) (string, *azure.Environment, error) {
	// load environment vars from $AZURE_CREDENTIALS_FILE, if it exists
	if err := loadEnv(); err != nil {
		return "", nil, err
	}

	// 1. we need AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID
	envVars, err := getRequiredValues(os.Getenv, tenantIDEnvVar, clientIDEnvVar, clientSecretEnvVar, subscriptionIDEnvVar)
	if err != nil {
		return "", nil, errors.Wrap(err, "unable to get all required environment variables")
	}

	// 2. Get Azure cloud from AZURE_CLOUD_NAME, if it exists. If the env var does not
	// exist, parseAzureEnvironment will return azure.PublicCloud.
	env, err := parseAzureEnvironment(os.Getenv(cloudNameEnvVar))
	if err != nil {
		return "", nil, errors.Wrap(err, "unable to parse azure cloud name environment variable")
	}

	// 3. check whether a different subscription ID was set for backups in config["subscriptionId"]
	subscriptionId := envVars[subscriptionIDEnvVar]
	if val := config[subscriptionIdConfigKey]; val != "" {
		subscriptionId = val
	}

	// 4. we need config["resourceGroup"], config["storageAccount"]
	if _, err := getRequiredValues(mapLookup(config), resourceGroupConfigKey, storageAccountConfigKey); err != nil {
		return "", env, errors.Wrap(err, "unable to get all required config values")
	}

	// 5. get SPT
	spt, err := newServicePrincipalToken(envVars[tenantIDEnvVar], envVars[clientIDEnvVar], envVars[clientSecretEnvVar], env)
	if err != nil {
		return "", env, errors.Wrap(err, "error getting service principal token")
	}

	// 6. get storageAccountsClient
	storageAccountsClient := storagemgmt.NewAccountsClientWithBaseURI(env.ResourceManagerEndpoint, subscriptionId)
	storageAccountsClient.Authorizer = autorest.NewBearerAuthorizer(spt)

	// 7. get storage key
	res, err := storageAccountsClient.ListKeys(context.TODO(), config[resourceGroupConfigKey], config[storageAccountConfigKey])
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

func newServicePrincipalToken(tenantID, clientID, clientSecret string, env *azure.Environment) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting OAuthConfig")
	}

	return adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, env.ResourceManagerEndpoint)
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
