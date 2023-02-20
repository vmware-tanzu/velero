/*
 *
 * Copyright the Velero contributors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package backups

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
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

type TTL struct {
	testNS      string
	backupName  string
	restoreName string
	ctx         context.Context
	ttl         time.Duration
}

func (b *TTL) Init() {
	rand.Seed(time.Now().UnixNano())
	UUIDgen, _ = uuid.NewRandom()
	b.testNS = "backup-ttl-test-" + UUIDgen.String()
	b.backupName = "backup-ttl-test-" + UUIDgen.String()
	b.restoreName = "restore-ttl-test-" + UUIDgen.String()
	b.ctx, _ = context.WithTimeout(context.Background(), time.Hour)
	b.ttl = 20 * time.Minute

}

func TTLTest() {
	var err error
	var veleroCfg VeleroConfig
	useVolumeSnapshots := true
	test := new(TTL)
	veleroCfg = VeleroCfg
	client := *veleroCfg.ClientToInstallVelero

	//Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests")

	BeforeEach(func() {
		flag.Parse()
		veleroCfg = VeleroCfg
		if veleroCfg.InstallVelero {
			// Make sure GCFrequency is shorter than backup TTL
			veleroCfg.GCFrequency = "4m0s"
			veleroCfg.UseVolumeSnapshots = useVolumeSnapshots
			Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
		}
	})

	AfterEach(func() {
		veleroCfg.GCFrequency = ""
		if !veleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *veleroCfg.ClientToInstallVelero)
			})
			if veleroCfg.InstallVelero {
				Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace)).To(Succeed())
			}
			Expect(DeleteNamespace(test.ctx, client, test.testNS, false)).To(Succeed(), fmt.Sprintf("Failed to delete the namespace %s", test.testNS))
		}
	})

	It("Backups in object storage should be synced to a new Velero successfully", func() {
		test.Init()
		By(fmt.Sprintf("Prepare workload as target to backup by creating namespace %s namespace", test.testNS), func() {
			Expect(CreateNamespace(test.ctx, client, test.testNS)).To(Succeed(),
				fmt.Sprintf("Failed to create %s namespace", test.testNS))
		})

		By("Deploy sample workload of Kibishii", func() {
			Expect(KibishiiPrepareBeforeBackup(test.ctx, client, veleroCfg.CloudProvider,
				test.testNS, veleroCfg.RegistryCredentialFile, veleroCfg.Features,
				veleroCfg.KibishiiDirectory, useVolumeSnapshots, DefaultKibishiiData)).To(Succeed())
		})

		var BackupCfg BackupConfig
		BackupCfg.BackupName = test.backupName
		BackupCfg.Namespace = test.testNS
		BackupCfg.BackupLocation = ""
		BackupCfg.UseVolumeSnapshots = useVolumeSnapshots
		BackupCfg.Selector = ""
		BackupCfg.TTL = test.ttl

		By(fmt.Sprintf("Backup the workload in %s namespace", test.testNS), func() {
			Expect(VeleroBackupNamespace(test.ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
				RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, test.backupName, "")
				return "Fail to backup workload"
			})
		})

		var snapshotCheckPoint SnapshotCheckPoint
		if useVolumeSnapshots {
			if veleroCfg.CloudProvider == "vsphere" {
				// TODO - remove after upload progress monitoring is implemented
				By("Waiting for vSphere uploads to complete", func() {
					Expect(WaitForVSphereUploadCompletion(test.ctx, time.Hour,
						test.testNS, 2)).To(Succeed())
				})
			}
			snapshotCheckPoint, err = GetSnapshotCheckPoint(client, veleroCfg, 2, test.testNS, test.backupName, KibishiiPodNameList)
			Expect(err).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")

			Expect(SnapshotsShouldBeCreatedInCloud(veleroCfg.CloudProvider,
				veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, veleroCfg.BSLConfig,
				test.backupName, snapshotCheckPoint)).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")
		}

		By(fmt.Sprintf("Simulating a disaster by removing namespace %s\n", BackupCfg.BackupName), func() {
			Expect(DeleteNamespace(test.ctx, client, BackupCfg.BackupName, true)).To(Succeed(),
				fmt.Sprintf("Failed to delete namespace %s", BackupCfg.BackupName))
		})

		if veleroCfg.CloudProvider == "aws" && useVolumeSnapshots {
			fmt.Println("Waiting 7 minutes to make sure the snapshots are ready...")
			time.Sleep(7 * time.Minute)
		}

		By(fmt.Sprintf("Restore %s", test.testNS), func() {
			Expect(VeleroRestore(test.ctx, veleroCfg.VeleroCLI,
				veleroCfg.VeleroNamespace, test.restoreName, test.backupName, "")).To(Succeed(), func() string {
				RunDebug(test.ctx, veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, "", test.restoreName)
				return "Fail to restore workload"
			})
		})

		By("Associated Restores should be created", func() {
			Expect(ObjectsShouldBeInBucket(veleroCfg.CloudProvider,
				veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
				veleroCfg.BSLPrefix, veleroCfg.BSLConfig, test.restoreName,
				RestoreObjectsPrefix)).NotTo(HaveOccurred(), "Fail to get restore object")

		})

		By("Check TTL was set correctly", func() {
			ttl, err := GetBackupTTL(test.ctx, veleroCfg.VeleroNamespace, test.backupName)
			Expect(err).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")
			t, _ := time.ParseDuration(strings.ReplaceAll(ttl, "'", ""))
			fmt.Println(t.Round(time.Minute).String())
			Expect(t).To(Equal(test.ttl))
		})

		By(fmt.Sprintf("Waiting %s minutes for removing backup ralated resources by GC", test.ttl.String()), func() {
			time.Sleep(test.ttl)
		})

		By("Check if backups are deleted by GC", func() {
			Expect(WaitBackupDeleted(test.ctx, veleroCfg.VeleroCLI, test.backupName, time.Minute*10)).To(Succeed(), fmt.Sprintf("Backup %s was not deleted by GC", test.backupName))
		})

		By("Backup file from cloud object storage should be deleted", func() {
			Expect(ObjectsShouldNotBeInBucket(veleroCfg.CloudProvider,
				veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
				veleroCfg.BSLPrefix, veleroCfg.BSLConfig, test.backupName,
				BackupObjectsPrefix, 5)).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")
		})

		By("PersistentVolume snapshots should be deleted", func() {
			if useVolumeSnapshots {
				Expect(SnapshotsShouldNotExistInCloud(veleroCfg.CloudProvider,
					veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, veleroCfg.BSLConfig,
					test.backupName, snapshotCheckPoint)).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")
			}
		})

		By("Associated Restores should be deleted", func() {
			Expect(ObjectsShouldNotBeInBucket(veleroCfg.CloudProvider,
				veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
				veleroCfg.BSLPrefix, veleroCfg.BSLConfig, test.restoreName,
				RestoreObjectsPrefix, 5)).NotTo(HaveOccurred(), "Fail to get restore object")

		})
	})
}
