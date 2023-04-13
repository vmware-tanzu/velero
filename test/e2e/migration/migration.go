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
var veleroCfg VeleroConfig

func MigrationWithSnapshots() {
	veleroCfg = VeleroCfg
	for _, veleroCLI2Version := range GetVersionList(veleroCfg.MigrateFromVeleroCLI, veleroCfg.MigrateFromVeleroVersion) {
		MigrationTest(true, veleroCLI2Version)
	}
}

func MigrationWithRestic() {
	veleroCfg = VeleroCfg
	for _, veleroCLI2Version := range GetVersionList(veleroCfg.MigrateFromVeleroCLI, veleroCfg.MigrateFromVeleroVersion) {
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
		veleroCfg = VeleroCfg
		UUIDgen, err = uuid.NewRandom()
		migrationNamespace = "migration-workload-" + UUIDgen.String()
		if useVolumeSnapshots && veleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		if useVolumeSnapshots && veleroCfg.CloudProvider == "aws" {
			Skip("Volume snapshots migration not supported on AWS provisioned by Sheperd public pool")
		}
		if veleroCfg.DefaultCluster == "" && veleroCfg.StandbyCluster == "" {
			Skip("Migration test needs 2 clusters")
		}
	})
	AfterEach(func() {
		if !veleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *veleroCfg.DefaultClient)
			})
			if veleroCfg.InstallVelero {
				By(fmt.Sprintf("Uninstall Velero and delete sample workload namespace %s", migrationNamespace), func() {
					Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace)).To(Succeed())
					DeleteNamespace(context.Background(), *veleroCfg.DefaultClient, migrationNamespace, true)

					Expect(KubectlConfigUseContext(context.Background(), veleroCfg.StandbyCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace)).To(Succeed())
					DeleteNamespace(context.Background(), *veleroCfg.StandbyClient, migrationNamespace, true)
				})
			}
			By(fmt.Sprintf("Switch to default kubeconfig context %s", veleroCfg.DefaultCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
				veleroCfg.ClientToInstallVelero = veleroCfg.DefaultClient
			})
		}

	})
	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			flag.Parse()
			UUIDgen, err = uuid.NewRandom()
			Expect(err).To(Succeed())
			supportUploaderType, err := IsSupportUploaderType(veleroCLI2Version.VeleroVersion)
			Expect(err).To(Succeed())
			oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)

			if veleroCLI2Version.VeleroCLI == "" {
				//Assume tag of velero server image is identical to velero CLI version
				//Download velero CLI if it's empty according to velero CLI version
				By(fmt.Sprintf("Install the expected version Velero CLI (%s) for installing Velero",
					veleroCLI2Version.VeleroVersion), func() {
					if veleroCLI2Version.VeleroVersion == "self" {
						veleroCLI2Version.VeleroCLI = veleroCfg.VeleroCLI
					} else {
						veleroCLI2Version.VeleroCLI, err = InstallVeleroCLI(veleroCLI2Version.VeleroVersion)
						Expect(err).To(Succeed())
					}
				})
			}
			OriginVeleroCfg := veleroCfg
			By(fmt.Sprintf("Install Velero in cluster-A (%s) to backup workload", veleroCfg.DefaultCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
				OriginVeleroCfg.MigrateFromVeleroVersion = veleroCLI2Version.VeleroVersion
				OriginVeleroCfg.VeleroCLI = veleroCLI2Version.VeleroCLI
				OriginVeleroCfg.ClientToInstallVelero = OriginVeleroCfg.DefaultClient
				OriginVeleroCfg.UseVolumeSnapshots = useVolumeSnapshots
				OriginVeleroCfg.UseNodeAgent = !useVolumeSnapshots
				// TODO: self means 1.10 and upper version
				if veleroCLI2Version.VeleroVersion != "self" {
					Expect(err).To(Succeed())
					fmt.Printf("Using default images address of Velero CLI %s\n", veleroCLI2Version.VeleroVersion)
					OriginVeleroCfg.VeleroImage = ""
					OriginVeleroCfg.RestoreHelperImage = ""
					OriginVeleroCfg.Plugins = ""
					//TODO: Remove this once origin Velero version is 1.10 and upper
					OriginVeleroCfg.UploaderType = ""
					if supportUploaderType {
						OriginVeleroCfg.UseRestic = false
						OriginVeleroCfg.UseNodeAgent = !useVolumeSnapshots
					} else {
						OriginVeleroCfg.UseRestic = !useVolumeSnapshots
						OriginVeleroCfg.UseNodeAgent = false
					}
				}

				Expect(VeleroInstall(context.Background(), &OriginVeleroCfg)).To(Succeed())
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
				Expect(CreateNamespace(oneHourTimeout, *veleroCfg.DefaultClient, migrationNamespace)).To(Succeed(),
					fmt.Sprintf("Failed to create namespace %s to install Kibishii workload", migrationNamespace))
			})

			By("Deploy sample workload of Kibishii", func() {
				Expect(KibishiiPrepareBeforeBackup(oneHourTimeout, *veleroCfg.DefaultClient, veleroCfg.CloudProvider,
					migrationNamespace, veleroCfg.RegistryCredentialFile, veleroCfg.Features,
					veleroCfg.KibishiiDirectory, useVolumeSnapshots, DefaultKibishiiData)).To(Succeed())
			})

			By(fmt.Sprintf("Backup namespace %s", migrationNamespace), func() {
				var BackupStorageClassCfg BackupConfig
				BackupStorageClassCfg.BackupName = backupScName
				BackupStorageClassCfg.IncludeResources = "StorageClass"
				BackupStorageClassCfg.IncludeClusterResources = true
				//TODO Remove UseRestic parameter once minor version is 1.10 or upper
				BackupStorageClassCfg.UseResticIfFSBackup = !supportUploaderType
				Expect(VeleroBackupNamespace(context.Background(), OriginVeleroCfg.VeleroCLI,
					OriginVeleroCfg.VeleroNamespace, BackupStorageClassCfg)).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, BackupStorageClassCfg.BackupName, "")
					return "Fail to backup workload"
				})

				var BackupCfg BackupConfig
				BackupCfg.BackupName = backupName
				BackupCfg.Namespace = migrationNamespace
				BackupCfg.UseVolumeSnapshots = useVolumeSnapshots
				BackupCfg.BackupLocation = ""
				BackupCfg.Selector = ""
				BackupCfg.DefaultVolumesToFsBackup = !useVolumeSnapshots
				//TODO Remove UseRestic parameter once minor version is 1.10 or upper
				BackupCfg.UseResticIfFSBackup = !supportUploaderType

				Expect(VeleroBackupNamespace(context.Background(), OriginVeleroCfg.VeleroCLI,
					OriginVeleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
					RunDebug(context.Background(), OriginVeleroCfg.VeleroCLI, OriginVeleroCfg.VeleroNamespace, BackupCfg.BackupName, "")
					return "Fail to backup workload"
				})
			})

			if useVolumeSnapshots {
				if veleroCfg.CloudProvider == "vsphere" {
					// TODO - remove after upload progress monitoring is implemented
					By("Waiting for vSphere uploads to complete", func() {
						Expect(WaitForVSphereUploadCompletion(context.Background(), time.Hour,
							migrationNamespace, 2)).To(Succeed())
					})
				}
				var snapshotCheckPoint SnapshotCheckPoint
				snapshotCheckPoint.NamespaceBackedUp = migrationNamespace
				By("Snapshot should be created in cloud object store", func() {
					snapshotCheckPoint, err := GetSnapshotCheckPoint(*veleroCfg.DefaultClient, veleroCfg, 2,
						migrationNamespace, backupName, KibishiiPodNameList)
					Expect(err).NotTo(HaveOccurred(), "Fail to get snapshot checkpoint")
					Expect(SnapshotsShouldBeCreatedInCloud(veleroCfg.CloudProvider,
						veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
						veleroCfg.BSLConfig, backupName, snapshotCheckPoint)).To(Succeed())
				})
			}

			if useVolumeSnapshots && veleroCfg.CloudProvider == "azure" && strings.EqualFold(veleroCfg.Features, "EnableCSI") {
				// Upgrade test is not running daily since no CSI plugin v1.0 released, because builds before
				//   v1.0 have issues to fail upgrade case.
				By("Sleep 5 minutes to avoid snapshot recreated by unknown reason ", func() {
					time.Sleep(5 * time.Minute)
				})
			}
			// the snapshots of AWS may be still in pending status when do the restore, wait for a while
			// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
			// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
			if (veleroCfg.CloudProvider == "aws" || veleroCfg.CloudProvider == "vsphere") && useVolumeSnapshots {
				fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
				time.Sleep(5 * time.Minute)
			}

			By(fmt.Sprintf("Install Velero in cluster-B (%s) to restore workload", veleroCfg.StandbyCluster), func() {
				ns, err := GetNamespace(context.Background(), *veleroCfg.DefaultClient, migrationNamespace)
				Expect(ns.Name).To(Equal(migrationNamespace))
				Expect(err).NotTo(HaveOccurred())

				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.StandbyCluster)).To(Succeed())
				_, err = GetNamespace(context.Background(), *veleroCfg.StandbyClient, migrationNamespace)
				Expect(err).To(HaveOccurred())
				strings.Contains(fmt.Sprint(err), "namespaces \""+migrationNamespace+"\" not found")

				fmt.Println(err)

				veleroCfg.ClientToInstallVelero = veleroCfg.StandbyClient
				veleroCfg.UseNodeAgent = !useVolumeSnapshots
				veleroCfg.UseRestic = false
				Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
			})

			By(fmt.Sprintf("Waiting for backups sync to Velero in cluster-B (%s)", veleroCfg.StandbyCluster), func() {
				Expect(WaitForBackupToBeCreated(context.Background(), veleroCfg.VeleroCLI, backupName, 5*time.Minute)).To(Succeed())
				Expect(WaitForBackupToBeCreated(context.Background(), veleroCfg.VeleroCLI, backupScName, 5*time.Minute)).To(Succeed())
			})

			By(fmt.Sprintf("Restore %s", migrationNamespace), func() {
				Expect(VeleroRestore(context.Background(), veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, restoreScName, backupScName, "StorageClass")).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
				Expect(VeleroRestore(context.Background(), veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, restoreName, backupName, "")).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
			})

			By(fmt.Sprintf("Verify workload %s after restore ", migrationNamespace), func() {
				Expect(KibishiiVerifyAfterRestore(*veleroCfg.StandbyClient, migrationNamespace,
					oneHourTimeout, DefaultKibishiiData)).To(Succeed(), "Fail to verify workload after restore")
			})
		})
	})
}
