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

package azure

import (
	"strings"

	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
)

const (
	tenantIDEnvVar       = "AZURE_TENANT_ID"
	subscriptionIDEnvVar = "AZURE_SUBSCRIPTION_ID"
	clientIDEnvVar       = "AZURE_CLIENT_ID"
	clientSecretEnvVar   = "AZURE_CLIENT_SECRET"

	resourceGroupConfigKey = "resourceGroup"
)

// GetResticEnvVars gets the environment variables that restic
// relies on (AZURE_ACCOUNT_NAME and AZURE_ACCOUNT_KEY) based
// on info in the provided object storage location config map.
func GetResticEnvVars(config map[string]string) (map[string]string, error) {
	storageAccountKey, err := getStorageAccountKey(config)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"AZURE_ACCOUNT_NAME": config[storageAccountConfigKey],
		"AZURE_ACCOUNT_KEY":  storageAccountKey,
	}, nil
}

func newServicePrincipalToken(tenantID, clientID, clientSecret, scope string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting OAuthConfig")
	}

	return adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, scope)
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
