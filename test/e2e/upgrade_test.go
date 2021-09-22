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
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

const (
	upgradeNamespace = "upgrade-workload"
)

// Upgrade test by Kibishi using restic
var _ = Describe("[Upgrade][Restic] Velero upgrade tests on cluster using the plugin provider for object storage and Restic for volume backups", backup_upgrade_restore_with_restic)

var _ = Describe("[Upgrade][Snapshot] Velero upgrade tests on cluster using the plugin provider for object storage and snapshots for volume backups", backup_upgrade_restore_with_snapshots)

func backup_upgrade_restore_with_snapshots() {
	backup_upgrade_restore_test(true)
}

func backup_upgrade_restore_with_restic() {
	backup_upgrade_restore_test(false)
}

func backup_upgrade_restore_test(useVolumeSnapshots bool) {
	var (
		backupName, restoreName string
	)
	upgradeFromVeleroCLI := upgradeFromVeleroCLI

	client, err := newTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests")

	BeforeEach(func() {
		if (len(upgradeFromVeleroVersion)) == 0 {
			Skip("An original velero version is required to run upgrade test, please run test with upgrade-from-velero-version=<version>")
		}
		if useVolumeSnapshots && cloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		//Assume tag of velero server image is identical to velero CLI version
		//Download velero CLI if it's empty according to velero CLI version
		if (len(upgradeFromVeleroCLI)) == 0 {
			upgradeFromVeleroCLI, err = installVeleroCLI(upgradeFromVeleroVersion)
			Expect(err).To(Succeed())
		}

		var err error
		flag.Parse()
		uuidgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if installVelero {
			//Set veleroImage and resticHelperImage to blank
			//veleroImage and resticHelperImage should be the default value in originalCli
			Expect(veleroInstall(context.Background(), upgradeFromVeleroCLI, "", "", "", veleroNamespace, cloudProvider, objectStoreProvider, useVolumeSnapshots,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, "", "", registryCredentialFile)).To(Succeed())
			Expect(checkVeleroVersion(context.Background(), upgradeFromVeleroCLI, upgradeFromVeleroVersion)).To(Succeed())
		} else {
			Skip("Upgrade test is skipped since user don't want to install any other velero")
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
			Expect(runUpgradeTests(client, veleroImage, veleroVersion, cloudProvider, upgradeFromVeleroCLI, veleroNamespace, backupName, restoreName, "", useVolumeSnapshots, registryCredentialFile)).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})
	})
}

// runUpgradeTests runs upgrade test on the provider by kibishii.
func runUpgradeTests(client testClient, upgradeToVeleroImage, upgradeToVeleroVersion, providerName, upgradeFromVeleroCLI, veleroNamespace, backupName, restoreName, backupLocation string,
	useVolumeSnapshots bool, registryCredentialFile string) error {
	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)
	if err := createNamespace(oneHourTimeout, client, upgradeNamespace); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", upgradeNamespace)
	}
	defer func() {
		if err := deleteNamespace(context.Background(), client, upgradeNamespace, true); err != nil {
			fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", upgradeNamespace))
		}
	}()
	if err := kibishiiPrepareBeforeBackup(oneHourTimeout, client, providerName, upgradeNamespace, registryCredentialFile); err != nil {
		return errors.Wrapf(err, "Failed to install and prepare data for kibishii %s", upgradeNamespace)
	}

	if err := veleroBackupNamespace(oneHourTimeout, upgradeFromVeleroCLI, veleroNamespace, backupName, upgradeNamespace, backupLocation, useVolumeSnapshots); err != nil {
		// TODO currently, the upgrade case covers the upgrade path from 1.6 to main and the velero v1.6 doesn't support "debug" command
		// TODO move to "runDebug" after we bump up to 1.7 in the upgrade case
		veleroBackupLogs(context.Background(), upgradeFromVeleroCLI, veleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", upgradeNamespace)
	}

	if providerName == "vsphere" && useVolumeSnapshots {
		// Wait for uploads started by the Velero Plug-in for vSphere to complete
		// TODO - remove after upload progress monitoring is implemented
		fmt.Println("Waiting for vSphere uploads to complete")
		if err := waitForVSphereUploadCompletion(oneHourTimeout, time.Hour, upgradeNamespace); err != nil {
			return errors.Wrapf(err, "Error waiting for uploads to complete")
		}
	}
	fmt.Printf("Simulating a disaster by removing namespace %s\n", upgradeNamespace)
	if err := deleteNamespace(oneHourTimeout, client, upgradeNamespace, true); err != nil {
		return errors.Wrapf(err, "failed to delete namespace %s", upgradeNamespace)
	}

	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	if providerName == "aws" && useVolumeSnapshots {
		fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
		time.Sleep(5 * time.Minute)
	}

	if err := veleroInstall(context.Background(), veleroCLI, upgradeToVeleroImage, resticHelperImage, plugins, veleroNamespace, cloudProvider, objectStoreProvider, useVolumeSnapshots,
		cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, crdsVersion, "", registryCredentialFile); err != nil {
		return errors.Wrapf(err, "Failed to install velero from image %s", upgradeToVeleroImage)
	}
	if err := checkVeleroVersion(context.Background(), veleroCLI, upgradeToVeleroVersion); err != nil {
		return errors.Wrapf(err, "Velero install version mismatch.")
	}
	if err := veleroRestore(oneHourTimeout, veleroCLI, veleroNamespace, restoreName, backupName); err != nil {
		runDebug(context.Background(), veleroCLI, veleroNamespace, "", restoreName)
		return errors.Wrapf(err, "Restore %s failed from backup %s", restoreName, backupName)
	}

	if err := kibishiiVerifyAfterRestore(client, upgradeNamespace, oneHourTimeout); err != nil {
		return errors.Wrapf(err, "Error verifying kibishii after restore")
	}

	fmt.Printf("Upgrade test completed successfully\n")
	return nil
}
