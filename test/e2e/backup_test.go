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
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	uuidgen uuid.UUID
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", backup_restore_with_restic)

var _ = Describe("[Snapshot] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", backup_restore_with_snapshots)

func backup_restore_with_snapshots() {
	backup_restore_test(true)
}

func backup_restore_with_restic() {
	backup_restore_test(false)
}

func backup_restore_test(useVolumeSnapshots bool) {
	var (
		backupName, restoreName string
	)

	client, err := newTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests")

	BeforeEach(func() {
		if useVolumeSnapshots && cloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		var err error
		flag.Parse()
		uuidgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if installVelero {
			Expect(veleroInstall(context.Background(), veleroCLI, veleroImage, resticHelperImage, plugins, veleroNamespace, cloudProvider, objectStoreProvider, useVolumeSnapshots,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, crdsVersion, "", registryCredentialFile)).To(Succeed())
		}
	})

	AfterEach(func() {
		if installVelero {
			err = veleroUninstall(context.Background(), veleroCLI, veleroNamespace)
			Expect(err).To(Succeed())
		}
	})

	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			backupName = "backup-" + uuidgen.String()
			restoreName = "restore-" + uuidgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.
			Expect(runKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, "", useVolumeSnapshots, registryCredentialFile)).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})

		It("should successfully back up and restore to an additional BackupStorageLocation with unique credentials", func() {
			if additionalBSLProvider == "" {
				Skip("no additional BSL provider given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if additionalBSLBucket == "" {
				Skip("no additional BSL bucket given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if additionalBSLCredentials == "" {
				Skip("no additional BSL credentials given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			Expect(veleroAddPluginsForProvider(context.TODO(), veleroCLI, veleroNamespace, additionalBSLProvider, addBSLPlugins)).To(Succeed())

			// Create Secret for additional BSL
			secretName := fmt.Sprintf("bsl-credentials-%s", uuidgen)
			secretKey := fmt.Sprintf("creds-%s", additionalBSLProvider)
			files := map[string]string{
				secretKey: additionalBSLCredentials,
			}

			Expect(createSecretFromFiles(context.TODO(), client, veleroNamespace, secretName, files)).To(Succeed())

			// Create additional BSL using credential
			additionalBsl := fmt.Sprintf("bsl-%s", uuidgen)
			Expect(veleroCreateBackupLocation(context.TODO(),
				veleroCLI,
				veleroNamespace,
				additionalBsl,
				additionalBSLProvider,
				additionalBSLBucket,
				additionalBSLPrefix,
				additionalBSLConfig,
				secretName,
				secretKey,
			)).To(Succeed())

			bsls := []string{"default", additionalBsl}

			for _, bsl := range bsls {
				backupName = fmt.Sprintf("backup-%s", bsl)
				restoreName = fmt.Sprintf("restore-%s", bsl)
				// We limit the length of backup name here to avoid the issue of vsphere plugin https://github.com/vmware-tanzu/velero-plugin-for-vsphere/issues/370
				// We can remove the logic once the issue is fixed
				if bsl == "default" {
					backupName = fmt.Sprintf("%s-%s", backupName, uuidgen)
					restoreName = fmt.Sprintf("%s-%s", restoreName, uuidgen)
				}

				Expect(runKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, bsl, useVolumeSnapshots, registryCredentialFile)).To(Succeed(),
					"Failed to successfully backup and restore Kibishii namespace using BSL %s", bsl)
			}
		})
	})
}
