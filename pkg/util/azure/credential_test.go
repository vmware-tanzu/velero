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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/require"
)

func TestNewCredential(t *testing.T) {
	options := policy.ClientOptions{}

	// no credentials
	creds := map[string]string{}
	_, err := NewCredential(creds, options)
	require.NotNil(t, err)

	// config credential
	creds = map[string]string{
		CredentialKeyTenantID:     "tenantid",
		CredentialKeyClientID:     "clientid",
		CredentialKeyClientSecret: "secret",
	}
	_, err = NewCredential(creds, options)
	require.Nil(t, err)
}

func Test_newConfigCredential(t *testing.T) {
	options := configCredentialOptions{}

	// tenantID not specified
	creds := map[string]string{}
	_, err := newConfigCredential(creds, options)
	require.NotNil(t, err)

	// clientID not specified
	creds = map[string]string{
		CredentialKeyTenantID: "clientid",
	}
	_, err = newConfigCredential(creds, options)
	require.NotNil(t, err)

	// client secret
	creds = map[string]string{
		CredentialKeyTenantID:     "clientid",
		CredentialKeyClientID:     "clientid",
		CredentialKeyClientSecret: "secret",
	}
	credential, err := newConfigCredential(creds, options)
	require.Nil(t, err)
	require.NotNil(t, credential)
	_, ok := credential.(*azidentity.ClientSecretCredential)
	require.True(t, ok)

	// client certificate
	creds = map[string]string{
		CredentialKeyTenantID:              "clientid",
		CredentialKeyClientID:              "clientid",
		CredentialKeyClientCertificatePath: "testdata/certificate.pem",
	}
	credential, err = newConfigCredential(creds, options)
	require.Nil(t, err)
	require.NotNil(t, credential)
	_, ok = credential.(*azidentity.ClientCertificateCredential)
	require.True(t, ok)

	// username/password
	creds = map[string]string{
		CredentialKeyTenantID: "clientid",
		CredentialKeyClientID: "clientid",
		CredentialKeyUsername: "username",
		CredentialKeyPassword: "password",
	}
	credential, err = newConfigCredential(creds, options)
	require.Nil(t, err)
	require.NotNil(t, credential)
	_, ok = credential.(*azidentity.UsernamePasswordCredential)
	require.True(t, ok)
}
