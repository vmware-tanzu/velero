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
	credentials, err := LoadCredentials(nil)
	require.NoError(t, err)
	assert.NotNil(t, credentials)

	// specified credential file in the config
	name := filepath.Join(os.TempDir(), "credential")
	file, err := os.Create(name)
	require.NoError(t, err)
	defer file.Close()
	defer os.Remove(name)
	_, err = file.WriteString("key: value")
	require.NoError(t, err)

	config := map[string]string{
		"credentialsFile": name,
	}
	credentials, err = LoadCredentials(config)
	require.NoError(t, err)
	assert.Equal(t, "value", credentials["key"])

	// use the default path defined via env variable
	config = nil
	os.Setenv("AZURE_CREDENTIALS_FILE", name)
	credentials, err = LoadCredentials(config)
	require.NoError(t, err)
	assert.Equal(t, "value", credentials["key"])
}

func TestGetClientOptions(t *testing.T) {
	// invalid cloud name
	bslCfg := map[string]string{}
	creds := map[string]string{
		CredentialKeyCloudName: "invalid",
	}
	_, err := GetClientOptions(bslCfg, creds)
	require.Error(t, err)

	// specify caCert
	bslCfg = map[string]string{
		CredentialKeyCloudName: "",
		"caCert":               "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUZTakNDQXpLZ0F3SUJBZ0lVWmcxbzRpWld2bVh5ekJrQ0J6SGdiODZGemtFd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1VqRUxNQWtHQTFVRUJoTUNRMDR4RERBS0JnTlZCQWdNQTFCRlN6RVJNQThHQTFVRUJ3d0lRbVZwSUVwcApibWN4RHpBTkJnTlZCQW9NQmxaTmQyRnlaVEVSTUE4R0ExVUVBd3dJU0dGeVltOXlRMEV3SGhjTk1qTXdPVEEyCk1ESXpOakUyV2hjTk1qUXdPVEExTURJek5qRTJXakJYTVFzd0NRWURWUVFHRXdKRFRqRU1NQW9HQTFVRUNBd0QKVUVWTE1SRXdEd1lEVlFRSERBaENaV2tnU21sdVp6RVBNQTBHQTFVRUNnd0dWazEzWVhKbE1SWXdGQVlEVlFRRApEQTFJWVhKaWIzSk5ZVzVoWjJWeU1JSUNJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBZzhBTUlJQ0NnS0NBZ0VBCnIrK1FHaHYvUnBDUTFIcncrMnYyQWNoaGhUVTVQL3hCd2RIWkZHWWJzMmxGbGtiL3oycEs2Y05ycFZmNUtmdjIKVUNpZEovMjFhZHc2SWNGZWxkSnFudU4rSlJaWXh5S0w0bzdRRGNVSk1sUTZJZk5kbEI0NUNwcGFBZVA4blVVTgo0YUwyV244b094L1pROTd2YmRXeERIR1FqZGR4N3p0Q09PaVZ0SEk4NS9Ka3kydTJnNmVhMklndmh1ZEVPZ3JtCjJzNU8zZlVtdHhSTEhwNnpDbURYZGZFUWg4ZFpndCs1d0RlazRWR2t4Zk81VG1tUHJ0LzBPTnVGYjJMWGVWV0sKeXkzVDFFTGNOSWhPSzQ1amhEejNnb2JhQzAwK0JTdzJMejVocXRxdUQ2RGxmME53TWtBQkt6d1dMZkROOXNrRApVazVYTmZNa2c0L3JhblRIWHlCNUNKSlk1akhORUtBQWlnM2NFSFNvejVjeGJqTE14VDhoMEk3MitEWldmUzFTCjU3Rm1SN2ZTNXk0QUcrU3Z1U3kvbktCUktJS2dQZ0t2Rjl6NktuL2ZPMTNsbk5LbHQwWU5mWFBFV2hZQytmMUoKTWpTOWc5eHBpYkZhM2Q0aHpOeWZhMWJHcUxtbkUwNVNpbWZNMVI1Z21Tamw1Z1FKQlltTHA3dWRLdjFDSUNRSwptQng2WG4vcnJEZHFiMndCRUNRSjBMbUo2SW5SaFZtT0s0WUdFeFRqZ1FRMldSWHYxMnhVK05GYWlZS3cxZkp0CkdaemFQeENxaG5JZXM5cGNPY0FjdmFHVngwSjlFYnRod0ltekdoTjBBREdCOVZaL1dFdHYvN0gwQ2xjOVlyT0gKNnRMb212b2pjQUZnN0xFbXZxeFFEOFFSTzlZZVdTTkgvV1REY1hVb3R5a0NBd0VBQWFNVE1CRXdEd1lEVlIwUgpCQWd3Qm9jRUNycFFxakFOQmdrcWhraUc5dzBCQVFzRkFBT0NBZ0VBZnRVdmR1UFMvajhSaWx4ME5aelhSeEY3Ck9HZW9qU2JaQ1ZvamNPdnRNTVlMaFkxdDE2Y2laY1VWMGF4Z2FUWkkvak9WMGJPTEl3dmEvcVB2Z1RmSWZqL0gKVzhiMlNTRVdIUzZPSFFaR1BYNy9zVFVwQzB6QVcva2haN1FWR1BoWEcyK0V1NjFaNE95ejZ5dTRPdi9MYjlMUQpmMU9zTXhwandkbmhxazFKaERxUkpZbGIvZ05TRGZnVlN0YmhHVzVhb3paUlBBMUtqVXVaT3QzR2xQR09Wd3ZLCnpUcFFMdGVTUHNibTJMcUl2ZEg4dlgzK1kwcHIzdEdtdnExbWtIWUhYQTlBZWtYRkVsRHc4dGtZVHdLaEFqblUKZEFjWTFkTis5ODNiMDI0L0JQUXZKQlRTVjd4blEyUnlrUmMrVGxIL3B5RlM1cEtVbUF0aU9qTElxL2ZEMmJVagorTzlxT1hjK0c1b0xEaXlXWDRXSG9XdkZZdTdva1gwT1dGcHFETXFOcHlLUkRzQ1FENXViMEVQaVlVS0hnWEhiCnV3UXVtK0pRRUREdzRXL1kzZktnMW9TWW1XOHJndFNPZmtRQlQ0UnlaTUg2SzN6cFp5dVVsbmJUV0NWeEcyYVoKWVo0T2JpbUFGbVlveGRYdktWdFU0YUdlTjRoaXBvb2dzaXVXKzZYQ3Bqa2pWZlZuUEY4elZVNlZ3anRQVkkzKwpxdWxRNWJLS3lKYng3bk9NNXFob2svSmk2N1pyZDhob3ZwclhhRUdvakNDTVI3MllPWGVuMlB3bVlZZWNkQ2pyCnErSDdHNUV3ZXBoRWxrN3RWRWY4RVV4OEc1Mk9SVEtZMkF1dlRGVlliUC8yaTROS1FlMWdEWWZrWnNzUk1MajEKK0JCQVVJcnFVMnRuUHhwZW4vMD0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=",
	}
	creds = map[string]string{}
	options, err := GetClientOptions(bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, options.Cloud, cloud.AzurePublic)
	assert.NotNil(t, options.Transport)

	// doesn't specify caCert
	bslCfg = map[string]string{
		CredentialKeyCloudName: "",
	}
	creds = map[string]string{}
	options, err = GetClientOptions(bslCfg, creds)
	require.NoError(t, err)
	assert.Equal(t, options.Cloud, cloud.AzurePublic)
	assert.Nil(t, options.Transport)
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
