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
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/kibishii"
	. "github.com/vmware-tanzu/velero/test/e2e/util/providers"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type UpgradeFromVelero struct {
	UpgradeFromVeleroVersion string
	UpgradeFromVeleroCLI     string
}

const (
	upgradeNamespace = "upgrade-workload"
)

func GetUpgradePathList() []UpgradeFromVelero {
	var upgradeFromVeleroList []UpgradeFromVelero
	UpgradeFromVeleroVersionList := strings.Split(VeleroCfg.UpgradeFromVeleroVersion, ",")
	UpgradeFromVeleroCliList := strings.Split(VeleroCfg.UpgradeFromVeleroCLI, ",")

	for _, upgradeFromVeleroVersion := range UpgradeFromVeleroVersionList {
		upgradeFromVeleroList = append(upgradeFromVeleroList,
			UpgradeFromVelero{upgradeFromVeleroVersion, ""})
	}
	for i, upgradeFromVeleroCli := range UpgradeFromVeleroCliList {
		if i == len(UpgradeFromVeleroVersionList)-1 {
			break
		}
		upgradeFromVeleroList[i].UpgradeFromVeleroCLI = upgradeFromVeleroCli
	}
	return upgradeFromVeleroList
}

func BackupUpgradeRestoreWithSnapshots() {
	for _, upgradeFromVelero := range GetUpgradePathList() {
		BackupUpgradeRestoreTest(true, upgradeFromVelero)
	}
}

func BackupUpgradeRestoreWithRestic() {
	for _, upgradeFromVelero := range GetUpgradePathList() {
		BackupUpgradeRestoreTest(false, upgradeFromVelero)
	}
}

func BackupUpgradeRestoreTest(useVolumeSnapshots bool, upgradeFromVelero UpgradeFromVelero) {
	var (
		backupName, restoreName string
		client                  TestClient
		err                     error
	)

	By("Create test client instance", func() {
		client, err = NewTestClient()
		Expect(err).NotTo(HaveOccurred(), "Failed to instantiate cluster client for backup tests")
	})
	BeforeEach(func() {
		if !VeleroCfg.InstallVelero {
			Skip("Upgrade test should not be triggered if VeleroCfg.InstallVelero is set to false")
		}
		if (len(VeleroCfg.UpgradeFromVeleroVersion)) == 0 {
			Skip("An original velero version is required to run upgrade test, please run test with upgrade-from-velero-version=<version>")
		}
		if useVolumeSnapshots && VeleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		if VeleroCfg.VeleroCLI == "" {
			Skip("VeleroCLI should be provide")
		}
	})
	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			if !VeleroCfg.Debug {
				By(fmt.Sprintf("Delete sample workload namespace %s", upgradeNamespace), func() {
					DeleteNamespace(context.Background(), client, upgradeNamespace, true)
				})
				By("Uninstall Velero", func() {
					Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace)).To(Succeed())
				})
			}
		}
	})
	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			flag.Parse()
			UUIDgen, err = uuid.NewRandom()
			Expect(err).To(Succeed())

			if upgradeFromVelero.UpgradeFromVeleroCLI == "" {
				//Assume tag of velero server image is identical to velero CLI version
				//Download velero CLI if it's empty according to velero CLI version
				By(fmt.Sprintf("Install the expected old version Velero CLI (%s) for installing Velero",
					upgradeFromVelero.UpgradeFromVeleroVersion), func() {
					upgradeFromVelero.UpgradeFromVeleroCLI, err = InstallVeleroCLI(upgradeFromVelero.UpgradeFromVeleroVersion)
					Expect(err).To(Succeed())
				})
			}
			VeleroCfg.GCFrequency = ""
			By(fmt.Sprintf("Install the expected old version Velero (%s) for upgrade",
				upgradeFromVelero.UpgradeFromVeleroVersion), func() {
				//Set VeleroImage and ResticHelperImage to blank
				//VeleroImage and ResticHelperImage should be the default value in originalCli
				tmpCfgForOldVeleroInstall := VeleroCfg
				tmpCfgForOldVeleroInstall.UpgradeFromVeleroVersion = upgradeFromVelero.UpgradeFromVeleroVersion
				tmpCfgForOldVeleroInstall.VeleroCLI = upgradeFromVelero.UpgradeFromVeleroCLI
				tmpCfgForOldVeleroInstall.VeleroImage = ""
				tmpCfgForOldVeleroInstall.ResticHelperImage = ""
				tmpCfgForOldVeleroInstall.Plugins = ""

				Expect(VeleroInstall(context.Background(), &tmpCfgForOldVeleroInstall,
					useVolumeSnapshots)).To(Succeed())
				Expect(CheckVeleroVersion(context.Background(), tmpCfgForOldVeleroInstall.VeleroCLI,
					tmpCfgForOldVeleroInstall.UpgradeFromVeleroVersion)).To(Succeed())
			})

			backupName = "backup-" + UUIDgen.String()
			restoreName = "restore-" + UUIDgen.String()
			tmpCfg := VeleroCfg
			tmpCfg.UpgradeFromVeleroCLI = upgradeFromVelero.UpgradeFromVeleroCLI
			tmpCfg.UpgradeFromVeleroVersion = upgradeFromVelero.UpgradeFromVeleroVersion
			oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)

			By("Create namespace for sample workload", func() {
				Expect(CreateNamespace(oneHourTimeout, client, upgradeNamespace)).To(Succeed(),
					fmt.Sprintf("Failed to create namespace %s to install Kibishii workload", upgradeNamespace))
			})

			By("Deploy sample workload of Kibishii", func() {
				Expect(KibishiiPrepareBeforeBackup(oneHourTimeout, client, tmpCfg.CloudProvider,
					upgradeNamespace, tmpCfg.RegistryCredentialFile, tmpCfg.Features,
					tmpCfg.KibishiiDirectory, useVolumeSnapshots)).To(Succeed())
			})

			By(fmt.Sprintf("Backup namespace %s", upgradeNamespace), func() {
				var BackupCfg BackupConfig
				BackupCfg.BackupName = backupName
				BackupCfg.Namespace = upgradeNamespace
				BackupCfg.BackupLocation = ""
				BackupCfg.UseVolumeSnapshots = useVolumeSnapshots
				BackupCfg.Selector = ""
				Expect(VeleroBackupNamespace(oneHourTimeout, tmpCfg.UpgradeFromVeleroCLI,
					tmpCfg.VeleroNamespace, BackupCfg)).ShouldNot(HaveOccurred(), func() string {
					err = VeleroBackupLogs(context.Background(), tmpCfg.UpgradeFromVeleroCLI,
						tmpCfg.VeleroNamespace, backupName)
					return "Get backup logs"
				})
			})

			if useVolumeSnapshots {
				if VeleroCfg.CloudProvider == "vsphere" {
					// TODO - remove after upload progress monitoring is implemented
					By("Waiting for vSphere uploads to complete", func() {
						Expect(WaitForVSphereUploadCompletion(oneHourTimeout, time.Hour,
							upgradeNamespace)).To(Succeed())
					})
				}
				var snapshotCheckPoint SnapshotCheckPoint
				snapshotCheckPoint.NamespaceBackedUp = upgradeNamespace
				By("Snapshot should be created in cloud object store", func() {
					snapshotCheckPoint, err := GetSnapshotCheckPoint(client, VeleroCfg, 2,
						upgradeNamespace, backupName, KibishiiPodNameList)
					Expect(err).NotTo(HaveOccurred(), "Fail to get snapshot checkpoint")
					Expect(SnapshotsShouldBeCreatedInCloud(VeleroCfg.CloudProvider,
						VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
						VeleroCfg.BSLConfig, backupName, snapshotCheckPoint)).To(Succeed())
				})
			}

			By(fmt.Sprintf("Simulating a disaster by removing namespace %s\n", upgradeNamespace), func() {
				Expect(DeleteNamespace(oneHourTimeout, client, upgradeNamespace, true)).To(Succeed(),
					fmt.Sprintf("failed to delete namespace %s", upgradeNamespace))
			})

			if useVolumeSnapshots && VeleroCfg.CloudProvider == "azure" && strings.EqualFold(VeleroCfg.Features, "EnableCSI") {
				// Upgrade test is not running daily since no CSI plugin v1.0 released, because builds before
				//   v1.0 have issues to fail upgrade case.
				By("Sleep 5 minutes to avoid snapshot recreated by unknown reason ", func() {
					time.Sleep(5 * time.Minute)
				})
			}
			// the snapshots of AWS may be still in pending status when do the restore, wait for a while
			// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
			// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
			if tmpCfg.CloudProvider == "aws" && useVolumeSnapshots {
				fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
				time.Sleep(5 * time.Minute)
			}

			By(fmt.Sprintf("Upgrade Velero by CLI %s", tmpCfg.VeleroCLI), func() {
				tmpCfg.GCFrequency = ""
				Expect(VeleroInstall(context.Background(), &tmpCfg, useVolumeSnapshots)).To(Succeed())
				Expect(CheckVeleroVersion(context.Background(), tmpCfg.VeleroCLI,
					tmpCfg.VeleroVersion)).To(Succeed())
			})

			By(fmt.Sprintf("Restore %s", upgradeNamespace), func() {
				Expect(VeleroRestore(oneHourTimeout, tmpCfg.VeleroCLI,
					tmpCfg.VeleroNamespace, restoreName, backupName)).To(Succeed(), func() string {
					RunDebug(context.Background(), tmpCfg.VeleroCLI,
						tmpCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
			})

			By(fmt.Sprintf("Verify workload %s after restore ", upgradeNamespace), func() {
				Expect(KibishiiVerifyAfterRestore(client, upgradeNamespace,
					oneHourTimeout)).To(Succeed(), "Fail to verify workload after restore")
			})
		})
	})
}
