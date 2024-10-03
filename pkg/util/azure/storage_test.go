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

package azure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorageClient(t *testing.T) {
	log := logrus.New()
	config := map[string]string{}

	name := filepath.Join(os.TempDir(), "credential")
	file, err := os.Create(name)
	require.NoError(t, err)
	defer file.Close()
	defer os.Remove(name)
	_, err = file.WriteString("AccessKey: YWNjZXNza2V5\nAZURE_TENANT_ID: tenantid\nAZURE_CLIENT_ID: clientid\nAZURE_CLIENT_SECRET: secret")
	require.NoError(t, err)

	// storage account isn't specified
	_, _, err = NewStorageClient(log, config)
	require.Error(t, err)

	// auth with storage account access key
	config = map[string]string{
		BSLConfigStorageAccount:              "storage-account",
		"credentialsFile":                    name,
		BSLConfigStorageAccountAccessKeyName: "AccessKey",
	}
	client, credential, err := NewStorageClient(log, config)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, credential)

	// auth with Azure AD
	config = map[string]string{
		BSLConfigStorageAccount: "storage-account",
		"credentialsFile":       name,
		"useAAD":                "true",
	}
	client, credential, err = NewStorageClient(log, config)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Nil(t, credential)
}

func TestGetStorageAccountCredentials(t *testing.T) {
	// use access secret but no secret specified
	cfg := map[string]string{
		BSLConfigStorageAccountAccessKeyName: "KEY",
	}
	creds := map[string]string{}
	_, err := GetStorageAccountCredentials(cfg, creds)
	require.Error(t, err)

	// use access secret
	cfg = map[string]string{
		BSLConfigStorageAccountAccessKeyName: "KEY",
	}
	creds = map[string]string{
		"KEY": "key",
	}
	m, err := GetStorageAccountCredentials(cfg, creds)
	require.NoError(t, err)
	assert.Equal(t, "key", m[CredentialKeyStorageAccountAccessKey])

	// use AAD, but useAAD invalid
	cfg = map[string]string{
		"useAAD": "invalid",
	}
	creds = map[string]string{}
	_, err = GetStorageAccountCredentials(cfg, creds)
	require.Error(t, err)

	// use AAD
	cfg = map[string]string{
		"useAAD": "true",
	}
	creds = map[string]string{
		"KEY": "key",
	}
	m, err = GetStorageAccountCredentials(cfg, creds)
	require.NoError(t, err)
	assert.Equal(t, creds, m)
}

func Test_getStorageAccountURI(t *testing.T) {
	log := logrus.New()

	// URI specified
	bslCfg := map[string]string{
		BSLConfigStorageAccountURI: "uri",
	}
	creds := map[string]string{}
	uri, err := getStorageAccountURI(log, bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, "uri", uri)

	// no URI specified, and auth with access key
	bslCfg = map[string]string{
		BSLConfigStorageAccountAccessKeyName: "KEY",
	}
	creds = map[string]string{
		"KEY": "value",
	}
	uri, err = getStorageAccountURI(log, bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, "https://.blob.core.windows.net", uri)

	// no URI specified, auth with AAD, resource group isn't specified
	bslCfg = map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
	}
	creds = map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	uri, err = getStorageAccountURI(log, bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, "https://.blob.core.windows.net", uri)

	// no URI specified, auth with AAD, resource group specified
	bslCfg = map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
		BSLConfigResourceGroup:  "resourcegroup",
		BSLConfigStorageAccount: "account",
	}
	creds = map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	uri, err = getStorageAccountURI(log, bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, "https://account.blob.core.windows.net", uri)
}

func Test_exchangeStorageAccountAccessKey(t *testing.T) {
	// resource group isn't specified
	bslCfg := map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
	}
	creds := map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	_, err := exchangeStorageAccountAccessKey(bslCfg, creds)
	require.Error(t, err)

	// storage account isn't specified
	bslCfg = map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
		BSLConfigResourceGroup:  "resourcegroup",
	}
	creds = map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	_, err = exchangeStorageAccountAccessKey(bslCfg, creds)
	require.Error(t, err)

	// storage account specified
	bslCfg = map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
		BSLConfigResourceGroup:  "resourcegroup",
		BSLConfigStorageAccount: "account",
	}
	creds = map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	_, err = exchangeStorageAccountAccessKey(bslCfg, creds)
	require.Error(t, err)
}

func Test_newStorageAccountManagemenClient(t *testing.T) {
	// subscription ID isn't specified
	bslCfg := map[string]string{}
	creds := map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	_, err := newStorageAccountManagemenClient(bslCfg, creds)
	require.Error(t, err)

	// subscription ID isn't specified
	bslCfg = map[string]string{
		BSLConfigSubscriptionID: "subscriptionid",
	}
	creds = map[string]string{
		"AZURE_TENANT_ID":     "tenantid",
		"AZURE_CLIENT_ID":     "clientid",
		"AZURE_CLIENT_SECRET": "secret",
	}
	_, err = newStorageAccountManagemenClient(bslCfg, creds)
	require.NoError(t, err)
}
