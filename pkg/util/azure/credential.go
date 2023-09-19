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
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/pkg/errors"
)

// NewCredential chains the config credential and workload identity credential
func NewCredential(creds map[string]string, options policy.ClientOptions) (azcore.TokenCredential, error) {
	var (
		credential []azcore.TokenCredential
		errMsgs    []string
	)

	additionalTenants := []string{}
	if tenants := creds[CredentialKeyAdditionallyAllowedTenants]; tenants != "" {
		additionalTenants = strings.Split(tenants, ";")
	}

	// config credential
	cfgCred, err := newConfigCredential(creds, configCredentialOptions{
		ClientOptions:              options,
		AdditionallyAllowedTenants: additionalTenants,
	})
	if err == nil {
		credential = append(credential, cfgCred)
	} else {
		errMsgs = append(errMsgs, err.Error())
	}

	// workload identity credential
	wic, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		AdditionallyAllowedTenants: additionalTenants,
		ClientOptions:              options,
	})
	if err == nil {
		credential = append(credential, wic)
	} else {
		errMsgs = append(errMsgs, err.Error())
	}

	if len(credential) == 0 {
		return nil, errors.Errorf("failed to create Azure credential: %s", strings.Join(errMsgs, "\n\t"))
	}

	return azidentity.NewChainedTokenCredential(credential, nil)
}

type configCredentialOptions struct {
	azcore.ClientOptions
	AdditionallyAllowedTenants []string
}

// newConfigCredential works same as the azidentity.EnvironmentCredential but reads the credentials from a map
// rather than environment variables. This is required for Velero to run B/R concurrently
// https://github.com/Azure/azure-sdk-for-go/blob/sdk/azidentity/v1.3.0/sdk/azidentity/environment_credential.go#L80
func newConfigCredential(creds map[string]string, options configCredentialOptions) (azcore.TokenCredential, error) {
	tenantID := creds[CredentialKeyTenantID]
	if tenantID == "" {
		return nil, errors.Errorf("%s is required", CredentialKeyTenantID)
	}
	clientID := creds[CredentialKeyClientID]
	if clientID == "" {
		return nil, errors.Errorf("%s is required", CredentialKeyClientID)
	}

	// client secret
	if clientSecret := creds[CredentialKeyClientSecret]; clientSecret != "" {
		return azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, &azidentity.ClientSecretCredentialOptions{
			AdditionallyAllowedTenants: options.AdditionallyAllowedTenants,
			ClientOptions:              options.ClientOptions,
		})
	}

	// certificate
	if certPath := creds[CredentialKeyClientCertificatePath]; certPath != "" {
		certData, err := os.ReadFile(certPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read certificate file %s", certPath)
		}
		var password []byte
		if v := creds[CredentialKeyClientCertificatePassword]; v != "" {
			password = []byte(v)
		}
		certs, key, err := azidentity.ParseCertificates(certData, password)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load certificate from %s", certPath)
		}
		o := &azidentity.ClientCertificateCredentialOptions{
			AdditionallyAllowedTenants: options.AdditionallyAllowedTenants,
			ClientOptions:              options.ClientOptions,
		}
		if v, ok := creds[CredentialKeySendCertChain]; ok {
			o.SendCertificateChain = v == "1" || strings.ToLower(v) == "true"
		}
		return azidentity.NewClientCertificateCredential(tenantID, clientID, certs, key, o)
	}

	// username/password
	if username := creds[CredentialKeyUsername]; username != "" {
		if password := creds[CredentialKeyPassword]; password != "" {
			return azidentity.NewUsernamePasswordCredential(tenantID, clientID, username, password,
				&azidentity.UsernamePasswordCredentialOptions{
					AdditionallyAllowedTenants: options.AdditionallyAllowedTenants,
					ClientOptions:              options.ClientOptions,
				})
		}
		return nil, errors.Errorf("%s is required", CredentialKeyPassword)
	}

	return nil, errors.New("incomplete credential configuration. Only AZURE_TENANT_ID and AZURE_CLIENT_ID are set")
}
