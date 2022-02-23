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

package e2e

import (
	"github.com/google/uuid"
)

var UUIDgen uuid.UUID

var VeleroCfg VerleroConfig

type VerleroConfig struct {
	VeleroCLI                string
	VeleroImage              string
	VeleroVersion            string
	CloudCredentialsFile     string
	BSLConfig                string
	BSLBucket                string
	BSLPrefix                string
	VSLConfig                string
	CloudProvider            string
	ObjectStoreProvider      string
	VeleroNamespace          string
	AdditionalBSLProvider    string
	AdditionalBSLBucket      string
	AdditionalBSLPrefix      string
	AdditionalBSLConfig      string
	AdditionalBSLCredentials string
	RegistryCredentialFile   string
	ResticHelperImage        string
	UpgradeFromVeleroVersion string
	UpgradeFromVeleroCLI     string
	Plugins                  string
	AddBSLPlugins            string
	InstallVelero            bool
}
