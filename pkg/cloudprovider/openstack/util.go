/*
Copyright 2019 the Heptio Ark contributors.

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

package openstack

import (
	"crypto/tls"
	"net/http"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/pkg/errors"
)

func getRegion() string {
	return os.Getenv("OS_REGION_NAME")
}

func authenticate() (*gophercloud.ProviderClient, error) {
	authOption, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pc, err := openstack.NewClient(authOption.IdentityEndpoint)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tlsconfig := &tls.Config{}
	tlsconfig.InsecureSkipVerify = true
	transport := &http.Transport{TLSClientConfig: tlsconfig}
	pc.HTTPClient = http.Client{
		Transport: transport,
	}

	err = openstack.Authenticate(pc, authOption)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return pc, nil
}
