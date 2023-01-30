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

//Refer to https://github.com/vmware-tanzu/velero/issues/4253
package backups

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/providers"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type SyncBackups struct {
	testNS     string
	backupName string
	ctx        context.Context
}

func (b *SyncBackups) Init() {
	rand.Seed(time.Now().UnixNano())
	UUIDgen, _ = uuid.NewRandom()
	b.testNS = "sync-bsl-test-" + UUIDgen.String()
	b.backupName = "sync-bsl-test-" + UUIDgen.String()
	b.ctx, _ = context.WithTimeout(context.Background(), time.Minute*10)
}

func BackupsSyncTest() {
	test := new(SyncBackups)
	var (
		err error
	)

	BeforeEach(func() {
		flag.Parse()
		if VeleroCfg.InstallVelero {
			veleroCfg := VeleroCfg
			veleroCfg.UseVolumeSnapshots = false
			Expect(VeleroInstall(context.Background(), &VeleroCfg)).To(Succeed())
		}
	})

	AfterEach(func() {
		if !VeleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *VeleroCfg.ClientToInstallVelero)
			})
			if VeleroCfg.InstallVelero {
				Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)).To(Succeed())
			}
		}

	})

	It("Backups in object storage should be synced to a new Velero successfully", func() {
		test.Init()
		By(fmt.Sprintf("Prepare workload as target to backup by creating namespace %s namespace", test.testNS))
		Expect(CreateNamespace(test.ctx, *VeleroCfg.ClientToInstallVelero, test.testNS)).To(Succeed(),
			fmt.Sprintf("Failed to create %s namespace", test.testNS))
		if !VeleroCfg.Debug {
			defer func() {
				Expect(DeleteNamespace(test.ctx, *VeleroCfg.ClientToInstallVelero, test.testNS, false)).To(Succeed(), fmt.Sprintf("Failed to delete the namespace %s", test.testNS))
			}()
		}

		var BackupCfg BackupConfig
		BackupCfg.BackupName = test.backupName
		BackupCfg.Namespace = test.testNS
		BackupCfg.BackupLocation = ""
		BackupCfg.UseVolumeSnapshots = false
		BackupCfg.Selector = ""
		By(fmt.Sprintf("Backup the workload in %s namespace", test.testNS), func() {
			Expect(VeleroBackupNamespace(test.ctx, VeleroCfg.VeleroCLI,
				VeleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
				RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, "")
				return "Fail to backup workload"
			})
		})

		By("Uninstall velero", func() {
			Expect(VeleroUninstall(test.ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)).To(Succeed())
		})

		By("Install velero", func() {
			veleroCfg := VeleroCfg
			veleroCfg.UseVolumeSnapshots = false
			Expect(VeleroInstall(test.ctx, &VeleroCfg)).To(Succeed())
		})

		By("Check all backups in object storage are synced to Velero", func() {
			Expect(test.IsBackupsSynced()).To(Succeed(), fmt.Sprintf("Failed to sync backup %s from object storage", test.backupName))
		})
	})

	It("Deleted backups in object storage are synced to be deleted in Velero", func() {
		test.Init()
		By(fmt.Sprintf("Prepare workload as target to backup by creating namespace in %s namespace", test.testNS), func() {
			Expect(CreateNamespace(test.ctx, *VeleroCfg.ClientToInstallVelero, test.testNS)).To(Succeed(),
				fmt.Sprintf("Failed to create %s namespace", test.testNS))
		})
		if !VeleroCfg.Debug {
			defer func() {
				Expect(DeleteNamespace(test.ctx, *VeleroCfg.ClientToInstallVelero, test.testNS, false)).To(Succeed(),
					fmt.Sprintf("Failed to delete the namespace %s", test.testNS))
			}()
		}
		var BackupCfg BackupConfig
		BackupCfg.BackupName = test.backupName
		BackupCfg.Namespace = test.testNS
		BackupCfg.BackupLocation = ""
		BackupCfg.UseVolumeSnapshots = false
		BackupCfg.Selector = ""
		By(fmt.Sprintf("Backup the workload in %s namespace", test.testNS), func() {
			Expect(VeleroBackupNamespace(test.ctx, VeleroCfg.VeleroCLI,
				VeleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
				RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, "")
				return "Fail to backup workload"
			})
		})

		By(fmt.Sprintf("Delete %s backup files in object store", test.backupName), func() {
			err = DeleteObjectsInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
				VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig, test.backupName, BackupObjectsPrefix)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to delete object in bucket %s with err %v", test.backupName, err))
		})

		By(fmt.Sprintf("Check %s backup files in object store is deleted", test.backupName), func() {
			err = ObjectsShouldNotBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
				VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig, test.backupName, BackupObjectsPrefix, 1)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to delete object in bucket %s with err %v", test.backupName, err))
		})

		By("Check if backups are deleted as a result of sync from BSL", func() {
			Expect(WaitBackupDeleted(test.ctx, VeleroCfg.VeleroCLI, test.backupName, time.Minute*10)).To(Succeed(), fmt.Sprintf("Failed to check backup %s deleted", test.backupName))
		})
	})
}

func (b *SyncBackups) IsBackupsSynced() error {
	return WaitForBackupToBeCreated(b.ctx, VeleroCfg.VeleroCLI, b.backupName, 10*time.Minute)
}
