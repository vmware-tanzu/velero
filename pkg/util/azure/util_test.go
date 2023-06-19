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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCredentials(t *testing.T) {
	// no credential file
	_, err := LoadCredentials(nil)
	require.NotNil(t, err)

	// specified credential file in the config
	name := filepath.Join(os.TempDir(), "credential")
	file, err := os.Create(name)
	require.Nil(t, err)
	defer file.Close()
	defer os.Remove(name)
	_, err = file.WriteString("key: value")
	require.Nil(t, err)

	config := map[string]string{
		"credentialsFile": name,
	}
	credentials, err := LoadCredentials(config)
	require.Nil(t, err)
	assert.Equal(t, "value", credentials["key"])

	// use the default path defined via env variable
	config = nil
	os.Setenv("AZURE_CREDENTIALS_FILE", name)
	credentials, err = LoadCredentials(config)
	require.Nil(t, err)
	assert.Equal(t, "value", credentials["key"])
}

func TestGetClientOptions(t *testing.T) {
	// invalid cloud name
	bslCfg := map[string]string{}
	creds := map[string]string{
		CredentialKeyCloudName: "invalid",
	}
	_, err := GetClientOptions(bslCfg, creds)
	require.NotNil(t, err)

	// valid
	bslCfg = map[string]string{
		CredentialKeyCloudName: "",
	}
	creds = map[string]string{}
	options, err := GetClientOptions(bslCfg, creds)
	require.Nil(t, err)
	assert.Equal(t, options.Cloud, cloud.AzurePublic)
}

func Test_getCloudConfiguration(t *testing.T) {
	publicCloudWithADURI := cloud.AzurePublic
	publicCloudWithADURI.ActiveDirectoryAuthorityHost = "https://example.com"
	cases := []struct {
		name     string
		bslCfg   map[string]string
		creds    map[string]string
		err      bool
		expected cloud.Configuration
	}{
		{
			name:   "invalid cloud name",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "invalid",
			},
			err: true,
		},
		{
			name:   "null cloud name",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "",
			},
			err:      false,
			expected: cloud.AzurePublic,
		},
		{
			name:   "azure public cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "AZURECLOUD",
			},
			err:      false,
			expected: cloud.AzurePublic,
		},
		{
			name:   "azure public cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "AZUREPUBLICCLOUD",
			},
			err:      false,
			expected: cloud.AzurePublic,
		},
		{
			name:   "azure public cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "azurecloud",
			},
			err:      false,
			expected: cloud.AzurePublic,
		},
		{
			name:   "azure China cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "AZURECHINACLOUD",
			},
			err:      false,
			expected: cloud.AzureChina,
		},
		{
			name:   "azure US government cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "AZUREUSGOVERNMENT",
			},
			err:      false,
			expected: cloud.AzureGovernment,
		},
		{
			name:   "azure US government cloud",
			bslCfg: map[string]string{},
			creds: map[string]string{
				CredentialKeyCloudName: "AZUREUSGOVERNMENTCLOUD",
			},
			err:      false,
			expected: cloud.AzureGovernment,
		},
		{
			name: "AD authority URI provided",
			bslCfg: map[string]string{
				BSLConfigActiveDirectoryAuthorityURI: "https://example.com",
			},
			creds: map[string]string{
				CredentialKeyCloudName: "",
			},
			err:      false,
			expected: publicCloudWithADURI,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := getCloudConfiguration(c.bslCfg, c.creds)
			require.Equal(t, c.err, err != nil)
			if !c.err {
				assert.Equal(t, c.expected, cfg)
			}
		})
	}
}

func TestGetFromLocationConfigOrCredential(t *testing.T) {
	// from cfg
	cfg := map[string]string{
		"cfgkey": "value",
	}
	creds := map[string]string{}
	cfgKey, credKey := "cfgkey", "credkey"
	str := GetFromLocationConfigOrCredential(cfg, creds, cfgKey, credKey)
	assert.Equal(t, "value", str)

	// from cred
	cfg = map[string]string{}
	creds = map[string]string{
		"credkey": "value",
	}
	str = GetFromLocationConfigOrCredential(cfg, creds, cfgKey, credKey)
	assert.Equal(t, "value", str)
}
