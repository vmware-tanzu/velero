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
package upgrade

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	. "github.com/vmware-tanzu/velero/test/e2e"
	k8sutils "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	kibishiiutils "github.com/vmware-tanzu/velero/test/e2e/util/kibishii"
	veleroutils "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

const (
	upgradeNamespace = "upgrade-workload"
)

func BackupUpgradeRestoreWithSnapshots() {
	BackupUpgradeRestoreTest(true)
}

func BackupUpgradeRestoreWithRestic() {
	BackupUpgradeRestoreTest(false)
}

func BackupUpgradeRestoreTest(useVolumeSnapshots bool) {
	var (
		backupName, restoreName, upgradeFromVeleroCLI string
	)

	client, err := k8sutils.NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests")

	BeforeEach(func() {
		if (len(VeleroCfg.UpgradeFromVeleroVersion)) == 0 {
			Skip("An original velero version is required to run upgrade test, please run test with upgrade-from-velero-version=<version>")
		}
		if useVolumeSnapshots && VeleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		//Assume tag of velero server image is identical to velero CLI version
		//Download velero CLI if it's empty according to velero CLI version
		if (len(VeleroCfg.UpgradeFromVeleroCLI)) == 0 {
			upgradeFromVeleroCLI, err = veleroutils.InstallVeleroCLI(VeleroCfg.UpgradeFromVeleroVersion)
			Expect(err).To(Succeed())
		}

		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if VeleroCfg.InstallVelero {
			//Set VeleroImage and ResticHelperImage to blank
			//VeleroImage and ResticHelperImage should be the default value in originalCli
			tmpCfg := VeleroCfg
			tmpCfg.VeleroCLI = upgradeFromVeleroCLI
			tmpCfg.VeleroImage = ""
			tmpCfg.ResticHelperImage = ""
			tmpCfg.Plugins = ""
			tmpCfg.CRDsVersion = ""
			Expect(veleroutils.VeleroInstall(context.Background(), &tmpCfg, "", useVolumeSnapshots)).To(Succeed())
			Expect(veleroutils.CheckVeleroVersion(context.Background(), upgradeFromVeleroCLI, tmpCfg.UpgradeFromVeleroVersion)).To(Succeed())
		} else {
			Skip("Upgrade test is skipped since user don't want to install any other velero")
		}
	})

	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			err = veleroutils.VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed())
		}
	})

	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			backupName = "backup-" + UUIDgen.String()
			restoreName = "restore-" + UUIDgen.String()
			Expect(runUpgradeTests(client, &VeleroCfg, upgradeFromVeleroCLI, backupName, restoreName, "", useVolumeSnapshots)).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})
	})
}

// runUpgradeTests runs upgrade test on the provider by kibishii.
func runUpgradeTests(client k8sutils.TestClient, veleroCfg *VerleroConfig, upgradeFromVeleroCLI, backupName, restoreName, backupLocation string,
	useVolumeSnapshots bool) error {
	if veleroCfg.VeleroCLI == "" {
		return errors.New("empty")
	}
	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)
	if err := k8sutils.CreateNamespace(oneHourTimeout, client, upgradeNamespace); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", upgradeNamespace)
	}
	defer func() {
		if err := k8sutils.DeleteNamespace(context.Background(), client, upgradeNamespace, true); err != nil {
			fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", upgradeNamespace))
		}
	}()
	if err := kibishiiutils.KibishiiPrepareBeforeBackup(oneHourTimeout, client, veleroCfg.CloudProvider, upgradeNamespace, veleroCfg.RegistryCredentialFile); err != nil {
		return errors.Wrapf(err, "Failed to install and prepare data for kibishii %s", upgradeNamespace)
	}

	if err := veleroutils.VeleroBackupNamespace(oneHourTimeout, upgradeFromVeleroCLI, veleroCfg.VeleroNamespace, backupName, upgradeNamespace, backupLocation, useVolumeSnapshots); err != nil {
		// TODO currently, the upgrade case covers the upgrade path from 1.6 to main and the velero v1.6 doesn't support "debug" command
		// TODO move to "veleroutils.RunDebug" after we bump up to 1.7 in the upgrade case
		veleroutils.VeleroBackupLogs(context.Background(), upgradeFromVeleroCLI, veleroCfg.VeleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", upgradeNamespace)
	}

	if veleroCfg.CloudProvider == "vsphere" && useVolumeSnapshots {
		// Wait for uploads started by the Velero Plug-in for vSphere to complete
		// TODO - remove after upload progress monitoring is implemented
		fmt.Println("Waiting for vSphere uploads to complete")
		if err := veleroutils.WaitForVSphereUploadCompletion(oneHourTimeout, time.Hour, upgradeNamespace); err != nil {
			return errors.Wrapf(err, "Error waiting for uploads to complete")
		}
	}
	fmt.Printf("Simulating a disaster by removing namespace %s\n", upgradeNamespace)
	if err := k8sutils.DeleteNamespace(oneHourTimeout, client, upgradeNamespace, true); err != nil {
		return errors.Wrapf(err, "failed to delete namespace %s", upgradeNamespace)
	}

	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	if veleroCfg.CloudProvider == "aws" && useVolumeSnapshots {
		fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
		time.Sleep(5 * time.Minute)
	}

	if err := veleroutils.VeleroInstall(context.Background(), veleroCfg, "", useVolumeSnapshots); err != nil {
		return errors.Wrapf(err, "Failed to install velero from image %s", veleroCfg.VeleroImage)
	}
	if err := veleroutils.CheckVeleroVersion(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroVersion); err != nil {
		return errors.Wrapf(err, "Velero install version mismatch.")
	}
	if err := veleroutils.VeleroRestore(oneHourTimeout, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, restoreName, backupName); err != nil {
		veleroutils.RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, "", restoreName)
		return errors.Wrapf(err, "Restore %s failed from backup %s", restoreName, backupName)
	}

	if err := kibishiiutils.KibishiiVerifyAfterRestore(client, upgradeNamespace, oneHourTimeout); err != nil {
		return errors.Wrapf(err, "Error verifying kibishii after restore")
	}

	fmt.Printf("Upgrade test completed successfully\n")
	return nil
}
