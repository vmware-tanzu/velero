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
package backups

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
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/kibishii"
	. "github.com/vmware-tanzu/velero/test/e2e/util/providers"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

const (
	deletionTest = "deletion-workload"
)

// Test backup and restore of Kibishi using restic

func Backup_deletion_with_snapshots() {
	backup_deletion_test(true)
}

func Backup_deletion_with_restic() {
	backup_deletion_test(false)
}
func backup_deletion_test(useVolumeSnapshots bool) {
	var (
		backupName string
	)

	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup deletion tests with error %v", err)

	BeforeEach(func() {
		if useVolumeSnapshots && VeleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed(), "Failed to generate uuid with error %v", err)
		if VeleroCfg.InstallVelero {
			err = VeleroInstall(context.Background(), &VeleroCfg, "", useVolumeSnapshots)
			Expect(err).To(Succeed(), "Failed to install velero with error %v", err)
		}
	})

	AfterEach(func() {
		if VeleroCfg.InstallVelero {
			err = VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)
			Expect(err).To(Succeed(), "Failed to uninstall velero with error %v", err)
		}
	})

	When("kibishii is the sample workload", func() {
		It("Deleted backups are deleted from object storage and backups deleted from object storage can be deleted locally", func() {
			backupName = "backup-" + UUIDgen.String()
			err = runBackupDeletionTests(client, VeleroCfg.VeleroCLI, VeleroCfg.CloudProvider, VeleroCfg.VeleroNamespace, backupName, "", useVolumeSnapshots, VeleroCfg.RegistryCredentialFile, VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig)
			Expect(err).To(Succeed(), "Failed to run backup deletion test with error %v", err)
		})
	})
}

// runUpgradeTests runs upgrade test on the provider by kibishii.
func runBackupDeletionTests(client TestClient, veleroCLI, providerName, veleroNamespace, backupName, backupLocation string,
	useVolumeSnapshots bool, registryCredentialFile, bslPrefix, bslConfig string) error {

	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)

	if err := CreateNamespace(oneHourTimeout, client, deletionTest); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", deletionTest)
	}
	defer func() {
		if err := DeleteNamespace(context.Background(), client, deletionTest, true); err != nil {
			fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", deletionTest))
		}
	}()

	if err := KibishiiPrepareBeforeBackup(oneHourTimeout, client, providerName, deletionTest, registryCredentialFile); err != nil {
		return errors.Wrapf(err, "Failed to install and prepare data for kibishii %s", deletionTest)
	}
	err := ObjectsShouldNotBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket, VeleroCfg.BSLPrefix, VeleroCfg.BSLConfig, backupName, BackupObjectsPrefix, 1)
	if err != nil {
		return err
	}
	if err := VeleroBackupNamespace(oneHourTimeout, veleroCLI, veleroNamespace, backupName, deletionTest, backupLocation, useVolumeSnapshots); err != nil {
		// TODO currently, the upgrade case covers the upgrade path from 1.6 to main and the velero v1.6 doesn't support "debug" command
		// TODO move to "runDebug" after we bump up to 1.7 in the upgrade case
		VeleroBackupLogs(context.Background(), VeleroCfg.UpgradeFromVeleroCLI, veleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", deletionTest)
	}

	if providerName == "vsphere" && useVolumeSnapshots {
		// Wait for uploads started by the Velero Plug-in for vSphere to complete
		// TODO - remove after upload progress monitoring is implemented
		fmt.Println("Waiting for vSphere uploads to complete")
		if err := WaitForVSphereUploadCompletion(oneHourTimeout, time.Hour, deletionTest); err != nil {
			return errors.Wrapf(err, "Error waiting for uploads to complete")
		}
	}
	err = ObjectsShouldBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix)
	if err != nil {
		return err
	}
	err = DeleteBackupResource(context.Background(), veleroCLI, backupName)
	if err != nil {
		return err
	}
	err = ObjectsShouldNotBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix, 5)
	if err != nil {
		fmt.Println(errors.Wrapf(err, "Failed to get object from bucket %q", backupName))
		return err
	}
	backupName = "backup-1-" + UUIDgen.String()
	if err := VeleroBackupNamespace(oneHourTimeout, veleroCLI, veleroNamespace, backupName, deletionTest, backupLocation, useVolumeSnapshots); err != nil {
		// TODO currently, the upgrade case covers the upgrade path from 1.6 to main and the velero v1.6 doesn't support "debug" command
		// TODO move to "runDebug" after we bump up to 1.7 in the upgrade case
		VeleroBackupLogs(context.Background(), VeleroCfg.UpgradeFromVeleroCLI, veleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", deletionTest)
	}
	err = DeleteObjectsInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix)
	if err != nil {
		fmt.Println(errors.Wrapf(err, "Failed to delete object in bucket %q", backupName))
		return err
	}
	err = ObjectsShouldNotBeInBucket(VeleroCfg.CloudProvider, VeleroCfg.CloudCredentialsFile, VeleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix, 1)
	if err != nil {
		return err
	}
	err = DeleteBackupResource(context.Background(), veleroCLI, backupName)
	if err != nil {
		fmt.Println(errors.Wrapf(err, "|| UNEXPECTED || - Failed to delete backup %q", backupName))
		return err
	}
	fmt.Printf("|| EXPECTED || - Backup deletion test completed successfully\n")
	return nil
}
