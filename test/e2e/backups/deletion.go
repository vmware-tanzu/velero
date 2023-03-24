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

const deletionTest = "deletion-workload"

// Test backup and restore of Kibishi using restic

func BackupDeletionWithSnapshots() {
	backup_deletion_test(true)
}

func BackupDeletionWithRestic() {
	backup_deletion_test(false)
}
func backup_deletion_test(useVolumeSnapshots bool) {
	var (
		backupName string
		err        error
		veleroCfg  VeleroConfig
	)
	veleroCfg = VeleroCfg
	veleroCfg.UseVolumeSnapshots = useVolumeSnapshots
	veleroCfg.UseNodeAgent = !useVolumeSnapshots

	BeforeEach(func() {
		if useVolumeSnapshots && veleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if veleroCfg.InstallVelero {
			Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
		}
	})

	AfterEach(func() {
		if !veleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *veleroCfg.ClientToInstallVelero)
			})
			if veleroCfg.InstallVelero {
				err = VeleroUninstall(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace)
				Expect(err).To(Succeed())
			}
		}
	})

	When("kibishii is the sample workload", func() {
		It("Deleted backups are deleted from object storage and backups deleted from object storage can be deleted locally", func() {
			backupName = "backup-" + UUIDgen.String()
			Expect(runBackupDeletionTests(*veleroCfg.ClientToInstallVelero, veleroCfg, backupName, "", useVolumeSnapshots, veleroCfg.KibishiiDirectory)).To(Succeed(),
				"Failed to run backup deletion test")
		})
	})
}

// runUpgradeTests runs upgrade test on the provider by kibishii.
func runBackupDeletionTests(client TestClient, veleroCfg VeleroConfig, backupName, backupLocation string,
	useVolumeSnapshots bool, kibishiiDirectory string) error {
	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)
	veleroCLI := veleroCfg.VeleroCLI
	providerName := veleroCfg.CloudProvider
	veleroNamespace := veleroCfg.VeleroNamespace
	registryCredentialFile := veleroCfg.RegistryCredentialFile
	bslPrefix := veleroCfg.BSLPrefix
	bslConfig := veleroCfg.BSLConfig
	veleroFeatures := veleroCfg.Features

	if err := CreateNamespace(oneHourTimeout, client, deletionTest); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", deletionTest)
	}
	if !veleroCfg.Debug {
		defer func() {
			if err := DeleteNamespace(context.Background(), client, deletionTest, true); err != nil {
				fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", deletionTest))
			}
		}()
	}

	if err := KibishiiPrepareBeforeBackup(oneHourTimeout, client, providerName, deletionTest,
		registryCredentialFile, veleroFeatures, kibishiiDirectory, useVolumeSnapshots, DefaultKibishiiData); err != nil {
		return errors.Wrapf(err, "Failed to install and prepare data for kibishii %s", deletionTest)
	}
	err := ObjectsShouldNotBeInBucket(veleroCfg.CloudProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, veleroCfg.BSLPrefix, veleroCfg.BSLConfig, backupName, BackupObjectsPrefix, 1)
	if err != nil {
		return err
	}
	var BackupCfg BackupConfig
	BackupCfg.BackupName = backupName
	BackupCfg.Namespace = deletionTest
	BackupCfg.BackupLocation = backupLocation
	BackupCfg.UseVolumeSnapshots = useVolumeSnapshots
	BackupCfg.DefaultVolumesToFsBackup = !useVolumeSnapshots
	BackupCfg.Selector = ""

	By(fmt.Sprintf("Back up workload with name %s", BackupCfg.BackupName), func() {
		Expect(VeleroBackupNamespace(oneHourTimeout, veleroCLI,
			veleroNamespace, BackupCfg)).To(Succeed(), func() string {
			RunDebug(context.Background(), veleroCLI, veleroNamespace, BackupCfg.BackupName, "")
			return "Fail to backup workload"
		})
	})

	if providerName == "vsphere" && useVolumeSnapshots {
		// Wait for uploads started by the Velero Plug-in for vSphere to complete
		// TODO - remove after upload progress monitoring is implemented
		fmt.Println("Waiting for vSphere uploads to complete")
		if err := WaitForVSphereUploadCompletion(oneHourTimeout, time.Hour, deletionTest, 2); err != nil {
			return errors.Wrapf(err, "Error waiting for uploads to complete")
		}
	}
	err = ObjectsShouldBeInBucket(veleroCfg.CloudProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix)
	if err != nil {
		return err
	}
	var snapshotCheckPoint SnapshotCheckPoint
	if useVolumeSnapshots {
		snapshotCheckPoint, err = GetSnapshotCheckPoint(client, veleroCfg, 2, deletionTest, backupName, KibishiiPodNameList)
		Expect(err).NotTo(HaveOccurred(), "Fail to get Azure CSI snapshot checkpoint")
		err = SnapshotsShouldBeCreatedInCloud(veleroCfg.CloudProvider,
			veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, bslConfig,
			backupName, snapshotCheckPoint)
		if err != nil {
			return errors.Wrap(err, "exceed waiting for snapshot created in cloud")
		}
	}
	err = DeleteBackupResource(context.Background(), veleroCLI, backupName)
	if err != nil {
		return err
	}
	if useVolumeSnapshots {
		err = SnapshotsShouldNotExistInCloud(veleroCfg.CloudProvider,
			veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, veleroCfg.BSLConfig,
			backupName, snapshotCheckPoint)
		if err != nil {
			return errors.Wrap(err, "exceed waiting for snapshot created in cloud")
		}
	}

	err = ObjectsShouldNotBeInBucket(veleroCfg.CloudProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix, 5)
	if err != nil {
		return err
	}
	if useVolumeSnapshots {
		if err := SnapshotsShouldNotExistInCloud(veleroCfg.CloudProvider,
			veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket,
			bslConfig, backupName, snapshotCheckPoint); err != nil {
			return errors.Wrap(err, "exceed waiting for snapshot created in cloud")
		}
	}
	backupName = "backup-1-" + UUIDgen.String()
	BackupCfg.BackupName = backupName

	By(fmt.Sprintf("Back up workload with name %s", BackupCfg.BackupName), func() {
		Expect(VeleroBackupNamespace(oneHourTimeout, veleroCLI,
			veleroNamespace, BackupCfg)).To(Succeed(), func() string {
			RunDebug(context.Background(), veleroCLI, veleroNamespace, BackupCfg.BackupName, "")
			return "Fail to backup workload"
		})
	})

	err = DeleteObjectsInBucket(veleroCfg.CloudProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix)
	if err != nil {
		return err
	}

	err = ObjectsShouldNotBeInBucket(veleroCfg.CloudProvider, veleroCfg.CloudCredentialsFile, veleroCfg.BSLBucket, bslPrefix, bslConfig, backupName, BackupObjectsPrefix, 1)
	if err != nil {
		return err
	}

	err = DeleteBackupResource(context.Background(), veleroCLI, backupName)
	if err != nil {
		return errors.Wrapf(err, "|| UNEXPECTED || - Failed to delete backup %q", backupName)
	} else {
		fmt.Printf("|| EXPECTED || - Success to delete backup %s locally\n", backupName)
	}
	fmt.Printf("|| EXPECTED || - Backup deletion test completed successfully\n")
	return nil
}
