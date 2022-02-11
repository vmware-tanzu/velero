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
	"k8s.io/apimachinery/pkg/util/wait"

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
	b.ctx, _ = context.WithTimeout(context.Background(), time.Duration(time.Minute*10))
}

func BackupsSyncTest() {
	test := new(SyncBackups)
	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests with error %v", err)

	BeforeEach(func() {
		flag.Parse()
		if VeleroCfg.InstallVelero {
			err = VeleroInstall(context.Background(), &VeleroCfg, "", false)
			Expect(err).To(Succeed(), "Failed to install velero with error %v", err)
		}
	})

	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			err = VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed(), "Failed to install velero with error %v", err)
		}
	})

	It("Backups in object storage should be synced to a new Velero successfully", func() {
		test.Init()
		By(fmt.Sprintf("Prepare workload as target to backup by creating namespace %s namespace", test.testNS), func() {
			err = CreateNamespace(test.ctx, client, test.testNS)
			Expect(err).To(Succeed(), "Failed to create %s namespace with error %v", test.testNS, err)
		})
		defer func() {
			err = DeleteNamespace(test.ctx, client, test.testNS, false)
			Expect(err).To(Succeed(), "Failed to delete the namespace %s with error %v", test.testNS, err)
		}()

		By(fmt.Sprintf("Backup the workload in %s namespace", test.testNS), func() {
			if err = VeleroBackupNamespace(test.ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, test.testNS, "", false); err != nil {
				RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, "")
			}
			Expect(err).To(Succeed(), "Failed to backup %s namespace with error %v", test.testNS, err)
		})

		By("Uninstall velero", func() {
			err = VeleroUninstall(test.ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed(), "Failed to uninstall velero with error %v", err)
		})

		By("Install velero", func() {
			VeleroCfg.ObjectStoreProvider = ""
			err = VeleroInstall(test.ctx, &VeleroCfg, "", false)
			Expect(err).To(Succeed(), "Failed to uninstall velero with error %v", err)
		})

		By("Check all backups in object storage are synced to Velero", func() {
			err = test.IsBackupsSynced()
			Expect(err).To(Succeed(), "Failed to sync backup %s from object storage with error %v ", test.backupName, err)
		})
	})

	It("Deleted backups in object storage are synced to be deleted in Velero", func() {
		test.Init()
		By(fmt.Sprintf("Prepare workload as target to backup by creating namespace in %s namespace", test.testNS), func() {
			err = CreateNamespace(test.ctx, client, test.testNS)
			Expect(err).To(Succeed(), "Failed to create %s namespace with error %v", test.testNS, err)
		})

		defer func() {
			err = DeleteNamespace(test.ctx, client, test.testNS, false)
			Expect(err).To(Succeed(), "Failed to delete the namespace %s with error %v ", test.testNS, err)
		}()

		By(fmt.Sprintf("Backup the workload in %s namespace", test.testNS), func() {
			if err = VeleroBackupNamespace(test.ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, test.testNS, "", false); err != nil {
				RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, test.backupName, "")
			}
			Expect(err).To(Succeed(), "Failed to backup %s namespace with error %v", test.testNS, err)
		})

		By(fmt.Sprintf("Delete %s backup files in object store", test.backupName), func() {
			err = DeleteObjectsInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
				VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig, test.backupName, BackupObjectsPrefix)
			Expect(err).To(Succeed(), "Failed to delete object in bucket %s with error %v", test.backupName, err)
		})

		By(fmt.Sprintf("Check %s backup files in object store is deleted", test.backupName), func() {
			err = ObjectsShouldNotBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket,
				VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig, test.backupName, BackupObjectsPrefix, 1)
			Expect(err).To(Succeed(), "Failed to delete object in bucket %s with error %v", test.backupName, err)
		})

		By("Check if backups are deleted as a result of sync from BSL", func() {
			err = WaitBackupDeleted(test.ctx, VeleroCfg.VeleroCLI, test.backupName, time.Minute*10)
			Expect(err).To(Succeed(), fmt.Sprintf("Failed to check backup %s deleted with error %v", test.backupName, err))
		})
	})
}

func (b *SyncBackups) IsBackupsSynced() error {
	return wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
		if exist, err := IsBackupExist(b.ctx, VeleroCfg.VeleroCLI, b.backupName); err != nil {
			return false, err
		} else {
			if exist {
				return true, nil
			} else {
				return false, nil
			}
		}
	})
}
