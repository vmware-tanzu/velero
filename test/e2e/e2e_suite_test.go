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

package e2e_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/backup"
	. "github.com/vmware-tanzu/velero/test/e2e/basic"
	. "github.com/vmware-tanzu/velero/test/e2e/scale"
	. "github.com/vmware-tanzu/velero/test/e2e/upgrade"
)

func init() {
	flag.StringVar(&VeleroCfg.CloudProvider, "cloud-provider", "", "cloud that Velero will be installed into.  Required.")
	flag.StringVar(&VeleroCfg.ObjectStoreProvider, "object-store-provider", "", "provider of object store plugin. Required if cloud-provider is kind, otherwise ignored.")
	flag.StringVar(&VeleroCfg.BSLBucket, "bucket", "", "name of the object storage bucket where backups from e2e tests should be stored. Required.")
	flag.StringVar(&VeleroCfg.CloudCredentialsFile, "credentials-file", "", "file containing credentials for backup and volume provider. Required.")
	flag.StringVar(&VeleroCfg.VeleroCLI, "velerocli", "velero", "path to the velero application to use.")
	flag.StringVar(&VeleroCfg.VeleroImage, "velero-image", "velero/velero:main", "image for the velero server to be tested.")
	flag.StringVar(&VeleroCfg.Plugins, "plugins", "", "provider plugins to be tested.")
	flag.StringVar(&VeleroCfg.AddBSLPlugins, "additional-bsl-plugins", "", "additional plugins to be tested.")
	flag.StringVar(&VeleroCfg.VeleroVersion, "velero-version", "main", "image version for the velero server to be tested with.")
	flag.StringVar(&VeleroCfg.ResticHelperImage, "restic-helper-image", "", "image for the velero restic restore helper to be tested.")
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroCLI, "upgrade-from-velero-cli", "", "path to the pre-upgrade velero application to use.")
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroVersion, "upgrade-from-velero-version", "v1.6.3", "image for the pre-upgrade velero server to be tested.")
	flag.StringVar(&VeleroCfg.BSLConfig, "bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.BSLPrefix, "prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	flag.StringVar(&VeleroCfg.VSLConfig, "vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.VeleroNamespace, "velero-namespace", "velero", "namespace to install Velero into")
	flag.BoolVar(&VeleroCfg.InstallVelero, "install-velero", true, "install/uninstall velero during the test.  Optional.")
	flag.StringVar(&VeleroCfg.RegistryCredentialFile, "registry-credential-file", "", "file containing credential for the image registry, follows the same format rules as the ~/.docker/config.json file. Optional.")
	// Flags to create an additional BSL for multiple credentials test
	flag.StringVar(&VeleroCfg.AdditionalBSLProvider, "additional-bsl-object-store-provider", "", "Provider of object store plugin for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLBucket, "additional-bsl-bucket", "", "name of the object storage bucket for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLPrefix, "additional-bsl-prefix", "", "prefix under which all Velero data should be stored within the bucket for additional backup storage location. Optional.")
	flag.StringVar(&VeleroCfg.AdditionalBSLConfig, "additional-bsl-config", "", "configuration to use for the additional backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.AdditionalBSLCredentials, "additional-bsl-credentials-file", "", "file containing credentials for additional backup storage location provider. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.CRDsVersion, "crds-version", "v1", "CRD apiVersion for velero CRD creation.")
}

var _ = Describe("[APIGroup] Velero tests with various CRD API group versions", APIGropuVersionsTest)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupRestoreWithRestic)

var _ = Describe("[Snapshot] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupRestoreWithSnapshots)

var _ = Describe("[Basic] Backup/restore of 2 namespaces", BasicBackupRestore)

var _ = Describe("[Scale] Backup/restore of 2500 namespaces", MultiNSBackupRestore)

// Upgrade test by Kibishi using restic
var _ = Describe("[Upgrade][Restic] Velero upgrade tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupUpgradeRestoreWithRestic)

var _ = Describe("[Upgrade][Snapshot] Velero upgrade tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupUpgradeRestoreWithSnapshots)

func TestE2e(t *testing.T) {
	// Skip running E2E tests when running only "short" tests because:
	// 1. E2E tests are long running tests involving installation of Velero and performing backup and restore operations.
	// 2. E2E tests require a kubernetes cluster to install and run velero which further requires ore configuration. See above referenced command line flags.
	if testing.Short() {
		t.Skip("Skipping E2E tests")
	}

	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("report.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "E2e Suite", []Reporter{junitReporter})
}
