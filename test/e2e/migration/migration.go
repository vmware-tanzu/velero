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
package migration

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

var migrationNamespace string

func MigrationWithSnapshots() {
	for _, veleroCLI2Version := range GetVersionList(VeleroCfg.MigrateFromVeleroCLI, VeleroCfg.MigrateFromVeleroVersion) {
		MigrationTest(true, veleroCLI2Version)
	}
}

func MigrationWithRestic() {
	for _, veleroCLI2Version := range GetVersionList(VeleroCfg.MigrateFromVeleroCLI, VeleroCfg.MigrateFromVeleroVersion) {
		MigrationTest(false, veleroCLI2Version)
	}
}

func MigrationTest(useVolumeSnapshots bool, veleroCLI2Version VeleroCLI2Version) {
	var (
		backupName, restoreName     string
		backupScName, restoreScName string
		err                         error
	)

	BeforeEach(func() {
		UUIDgen, err = uuid.NewRandom()
		migrationNamespace = "migration-workload-" + UUIDgen.String()
		if useVolumeSnapshots && VeleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		if useVolumeSnapshots && VeleroCfg.CloudProvider == "aws" {
			Skip("Volume snapshots migration not supported on AWS provisioned by Sheperd public pool")
		}
		if VeleroCfg.DefaultCluster == "" && VeleroCfg.StandbyCluster == "" {
			Skip("Migration test needs 2 clusters")
		}
	})
	AfterEach(func() {
		if !VeleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *VeleroCfg.DefaultClient)
			})
			if VeleroCfg.InstallVelero {
				By(fmt.Sprintf("Uninstall Velero and delete sample workload namespace %s", migrationNamespace), func() {
					Expect(KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace)).To(Succeed())
					DeleteNamespace(context.Background(), *VeleroCfg.DefaultClient, migrationNamespace, true)

					Expect(KubectlConfigUseContext(context.Background(), VeleroCfg.StandbyCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace)).To(Succeed())
					DeleteNamespace(context.Background(), *VeleroCfg.StandbyClient, migrationNamespace, true)
				})
			}
			By(fmt.Sprintf("Switch to default kubeconfig context %s", VeleroCfg.DefaultClient), func() {
				Expect(KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultCluster)).To(Succeed())
				VeleroCfg.ClientToInstallVelero = VeleroCfg.DefaultClient
			})
		}

	})
	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			flag.Parse()
			UUIDgen, err = uuid.NewRandom()
			Expect(err).To(Succeed())

			oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*10)

			if veleroCLI2Version.VeleroCLI == "" {
				//Assume tag of velero server image is identical to velero CLI version
				//Download velero CLI if it's empty according to velero CLI version
				By(fmt.Sprintf("Install the expected version Velero CLI (%s) for installing Velero",
					veleroCLI2Version.VeleroVersion), func() {
					if veleroCLI2Version.VeleroVersion == "self" {
						veleroCLI2Version.VeleroCLI = VeleroCfg.VeleroCLI
					} else {
						veleroCLI2Version.VeleroCLI, err = InstallVeleroCLI(veleroCLI2Version.VeleroVersion)
						Expect(err).To(Succeed())
					}
				})
			}

			By(fmt.Sprintf("Install Velero in cluster-A (%s) to backup workload", VeleroCfg.DefaultCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultCluster)).To(Succeed())

				OriginVeleroCfg := VeleroCfg
				OriginVeleroCfg.MigrateFromVeleroVersion = veleroCLI2Version.VeleroVersion
				OriginVeleroCfg.VeleroCLI = veleroCLI2Version.VeleroCLI
				OriginVeleroCfg.ClientToInstallVelero = OriginVeleroCfg.DefaultClient
				if veleroCLI2Version.VeleroVersion != "self" {
					fmt.Printf("Using default images address of Velero CLI %s\n", veleroCLI2Version.VeleroVersion)
					OriginVeleroCfg.VeleroImage = ""
					OriginVeleroCfg.ResticHelperImage = ""
					OriginVeleroCfg.Plugins = ""
				}
				fmt.Println(OriginVeleroCfg)
				Expect(VeleroInstall(context.Background(), &OriginVeleroCfg, useVolumeSnapshots)).To(Succeed())
				if veleroCLI2Version.VeleroVersion != "self" {
					Expect(CheckVeleroVersion(context.Background(), OriginVeleroCfg.VeleroCLI,
						OriginVeleroCfg.MigrateFromVeleroVersion)).To(Succeed())
				}
			})

			backupName = "backup-" + UUIDgen.String()
			backupScName = backupName + "-sc"
			restoreName = "restore-" + UUIDgen.String()
			restoreScName = restoreName + "-sc"

			By("Create namespace for sample workload", func() {
				Expect(CreateNamespace(oneHourTimeout, *VeleroCfg.DefaultClient, migrationNamespace)).To(Succeed(),
					fmt.Sprintf("Failed to create namespace %s to install Kibishii workload", migrationNamespace))
			})

			By("Deploy sample workload of Kibishii", func() {
				Expect(KibishiiPrepareBeforeBackup(oneHourTimeout, *VeleroCfg.DefaultClient, VeleroCfg.CloudProvider,
					migrationNamespace, VeleroCfg.RegistryCredentialFile, VeleroCfg.Features,
					VeleroCfg.KibishiiDirectory, useVolumeSnapshots, DefaultKibishiiData)).To(Succeed())
			})

			By(fmt.Sprintf("Backup namespace %s", migrationNamespace), func() {
				var BackupStorageClassCfg BackupConfig
				BackupStorageClassCfg.BackupName = backupScName
				BackupStorageClassCfg.IncludeResources = "StorageClass"
				BackupStorageClassCfg.IncludeClusterResources = true
				Expect(VeleroBackupNamespace(context.Background(), VeleroCfg.VeleroCLI,
					VeleroCfg.VeleroNamespace, BackupStorageClassCfg)).ShouldNot(HaveOccurred(), func() string {
					err = VeleroBackupLogs(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace, backupName)
					return "Get backup logs"
				})

				var BackupCfg BackupConfig
				BackupCfg.BackupName = backupName
				BackupCfg.Namespace = migrationNamespace
				BackupCfg.UseVolumeSnapshots = useVolumeSnapshots
				BackupCfg.BackupLocation = ""
				BackupCfg.Selector = ""
				//BackupCfg.ExcludeResources = "tierentitlementbindings,tierentitlements,tiers,capabilities,customresourcedefinitions"
				Expect(VeleroBackupNamespace(context.Background(), VeleroCfg.VeleroCLI,
					VeleroCfg.VeleroNamespace, BackupCfg)).ShouldNot(HaveOccurred(), func() string {
					err = VeleroBackupLogs(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace, backupName)
					return "Get backup logs"
				})

			})

			if useVolumeSnapshots {
				if VeleroCfg.CloudProvider == "vsphere" {
					// TODO - remove after upload progress monitoring is implemented
					By("Waiting for vSphere uploads to complete", func() {
						Expect(WaitForVSphereUploadCompletion(context.Background(), time.Hour,
							migrationNamespace)).To(Succeed())
					})
				}
				var snapshotCheckPoint SnapshotCheckPoint
				snapshotCheckPoint.NamespaceBackedUp = migrationNamespace
				By("Snapshot should be created in cloud object store", func() {
					snapshotCheckPoint, err := GetSnapshotCheckPoint(*VeleroCfg.DefaultClient, VeleroCfg, 2,
						migrationNamespace, backupName, KibishiiPodNameList)
					Expect(err).NotTo(HaveOccurred(), "Fail to get snapshot checkpoint")
					Expect(SnapshotsShouldBeCreatedInCloud(VeleroCfg.CloudProvider,
						VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
						VeleroCfg.BSLConfig, backupName, snapshotCheckPoint)).To(Succeed())
				})
			}

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
			if VeleroCfg.CloudProvider == "aws" && useVolumeSnapshots {
				fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
				time.Sleep(5 * time.Minute)
			}

			By(fmt.Sprintf("Install Velero in cluster-B (%s) to restore workload", VeleroCfg.StandbyCluster), func() {
				ns, err := GetNamespace(context.Background(), *VeleroCfg.DefaultClient, migrationNamespace)
				Expect(ns.Name).To(Equal(migrationNamespace))
				Expect(err).NotTo(HaveOccurred())

				Expect(KubectlConfigUseContext(context.Background(), VeleroCfg.StandbyCluster)).To(Succeed())
				_, err = GetNamespace(context.Background(), *VeleroCfg.StandbyClient, migrationNamespace)
				Expect(err).To(HaveOccurred())
				strings.Contains(fmt.Sprint(err), "namespaces \""+migrationNamespace+"\" not found")

				fmt.Println(err)

				VeleroCfg.ObjectStoreProvider = ""
				VeleroCfg.ClientToInstallVelero = VeleroCfg.StandbyClient
				Expect(VeleroInstall(context.Background(), &VeleroCfg, useVolumeSnapshots)).To(Succeed())
			})

			By(fmt.Sprintf("Waiting for backups sync to Velero in cluster-B (%s)", VeleroCfg.StandbyCluster), func() {
				Expect(WaitForBackupToBeCreated(context.Background(), VeleroCfg.VeleroCLI, backupName, 5*time.Minute)).To(Succeed())
				Expect(WaitForBackupToBeCreated(context.Background(), VeleroCfg.VeleroCLI, backupScName, 5*time.Minute)).To(Succeed())
			})

			By(fmt.Sprintf("Restore %s", migrationNamespace), func() {
				Expect(VeleroRestore(context.Background(), VeleroCfg.VeleroCLI,
					VeleroCfg.VeleroNamespace, restoreScName, backupScName, "StorageClass")).To(Succeed(), func() string {
					RunDebug(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
				Expect(VeleroRestore(context.Background(), VeleroCfg.VeleroCLI,
					VeleroCfg.VeleroNamespace, restoreName, backupName, "")).To(Succeed(), func() string {
					RunDebug(context.Background(), VeleroCfg.VeleroCLI,
						VeleroCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
			})

			By(fmt.Sprintf("Verify workload %s after restore ", migrationNamespace), func() {
				Expect(KibishiiVerifyAfterRestore(*VeleroCfg.StandbyClient, migrationNamespace,
					oneHourTimeout, DefaultKibishiiData)).To(Succeed(), "Fail to verify workload after restore")
			})
		})
	})
}
