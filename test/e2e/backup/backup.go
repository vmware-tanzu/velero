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
package backup

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/kibishii"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

func BackupRestoreWithSnapshots() {
	BackupRestoreTest(true)
}

func BackupRestoreWithRestic() {
	BackupRestoreTest(false)
}

func BackupRestoreTest(useVolumeSnapshots bool) {

	var (
		backupName, restoreName, kibishiiNamespace string
		err                                        error
		provideSnapshotVolumesParmInBackup         bool
		veleroCfg                                  VeleroConfig
	)
	provideSnapshotVolumesParmInBackup = false

	BeforeEach(func() {
		veleroCfg = VeleroCfg
		veleroCfg.UseVolumeSnapshots = useVolumeSnapshots
		veleroCfg.UseNodeAgent = !useVolumeSnapshots
		if useVolumeSnapshots && veleroCfg.CloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		kibishiiNamespace = "kibishii-workload" + UUIDgen.String()
		Expect(err).To(Succeed())
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
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			// TODO[High] - remove code block below when vSphere plugin PR #500 is included in release version.
			//  because restore will be partiallyFailed when DefaultVolumesToFsBackup is set to true during
			//  Velero installation with default BSL.
			if veleroCfg.CloudProvider == "vsphere" && !useVolumeSnapshots {
				Skip("vSphere plugin PR #500 is not included in latest version 1.4.2")
			}

			if veleroCfg.InstallVelero {
				if useVolumeSnapshots {
					//Install node agent also
					veleroCfg.UseNodeAgent = useVolumeSnapshots
					veleroCfg.DefaultVolumesToFsBackup = useVolumeSnapshots
				} else {
					veleroCfg.DefaultVolumesToFsBackup = !useVolumeSnapshots
				}
				Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
			}
			backupName = "backup-" + UUIDgen.String()
			restoreName = "restore-" + UUIDgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.

			// if set ProvideSnapshotsVolumeParam to false here, make sure set it true in other tests of this case
			veleroCfg.ProvideSnapshotsVolumeParam = provideSnapshotVolumesParmInBackup

			// Set DefaultVolumesToFsBackup to false since DefaultVolumesToFsBackup was set to true during installation
			Expect(RunKibishiiTests(veleroCfg, backupName, restoreName, "", kibishiiNamespace, useVolumeSnapshots, false)).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})

		It("should successfully back up and restore to an additional BackupStorageLocation with unique credentials", func() {
			if veleroCfg.AdditionalBSLProvider == "" {
				Skip("no additional BSL provider given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if veleroCfg.AdditionalBSLBucket == "" {
				Skip("no additional BSL bucket given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if veleroCfg.AdditionalBSLCredentials == "" {
				Skip("no additional BSL credentials given, not running multiple BackupStorageLocation with unique credentials tests")
			}
			if veleroCfg.InstallVelero {
				if useVolumeSnapshots {
					veleroCfg.DefaultVolumesToFsBackup = !useVolumeSnapshots
				} else {
					// Install VolumeSnapshots also
					veleroCfg.UseVolumeSnapshots = !useVolumeSnapshots
					veleroCfg.DefaultVolumesToFsBackup = useVolumeSnapshots
				}

				Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
			}

			Expect(VeleroAddPluginsForProvider(context.TODO(), veleroCfg.VeleroCLI,
				veleroCfg.VeleroNamespace, veleroCfg.AdditionalBSLProvider,
				veleroCfg.AddBSLPlugins, veleroCfg.Features)).To(Succeed())

			// Create Secret for additional BSL
			secretName := fmt.Sprintf("bsl-credentials-%s", UUIDgen)
			secretKey := fmt.Sprintf("creds-%s", veleroCfg.AdditionalBSLProvider)
			files := map[string]string{
				secretKey: veleroCfg.AdditionalBSLCredentials,
			}

			Expect(CreateSecretFromFiles(context.TODO(), *veleroCfg.ClientToInstallVelero, veleroCfg.VeleroNamespace, secretName, files)).To(Succeed())

			// Create additional BSL using credential
			additionalBsl := fmt.Sprintf("bsl-%s", UUIDgen)
			Expect(VeleroCreateBackupLocation(context.TODO(),
				veleroCfg.VeleroCLI,
				veleroCfg.VeleroNamespace,
				additionalBsl,
				veleroCfg.AdditionalBSLProvider,
				veleroCfg.AdditionalBSLBucket,
				veleroCfg.AdditionalBSLPrefix,
				veleroCfg.AdditionalBSLConfig,
				secretName,
				secretKey,
			)).To(Succeed())

			bsls := []string{"default", additionalBsl}

			for _, bsl := range bsls {
				backupName = fmt.Sprintf("backup-%s", bsl)
				restoreName = fmt.Sprintf("restore-%s", bsl)
				// We limit the length of backup name here to avoid the issue of vsphere plugin https://github.com/vmware-tanzu/velero-plugin-for-vsphere/issues/370
				// We can remove the logic once the issue is fixed
				if bsl == "default" {
					backupName = fmt.Sprintf("%s-%s", backupName, UUIDgen)
					restoreName = fmt.Sprintf("%s-%s", restoreName, UUIDgen)
				}
				veleroCfg.ProvideSnapshotsVolumeParam = !provideSnapshotVolumesParmInBackup
				Expect(RunKibishiiTests(veleroCfg, backupName, restoreName, bsl, kibishiiNamespace, useVolumeSnapshots, !useVolumeSnapshots)).To(Succeed(),
					"Failed to successfully backup and restore Kibishii namespace using BSL %s", bsl)
			}
		})
	})
}
