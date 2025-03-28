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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vmware-tanzu/velero/test"
	framework "github.com/vmware-tanzu/velero/test/e2e/test"
	util "github.com/vmware-tanzu/velero/test/util/csi"
	k8sutil "github.com/vmware-tanzu/velero/test/util/k8s"
	"github.com/vmware-tanzu/velero/test/util/kibishii"
	"github.com/vmware-tanzu/velero/test/util/providers"
	veleroutil "github.com/vmware-tanzu/velero/test/util/velero"
)

type migrationE2E struct {
	framework.TestCase
	useVolumeSnapshots bool
	veleroCLI2Version  test.VeleroCLI2Version
	kibishiiData       kibishii.KibishiiData
}

func MigrationWithSnapshots() {
	for _, veleroCLI2Version := range veleroutil.GetVersionList(
		test.VeleroCfg.MigrateFromVeleroCLI,
		test.VeleroCfg.MigrateFromVeleroVersion) {
		framework.TestFunc(
			&migrationE2E{
				useVolumeSnapshots: true,
				veleroCLI2Version:  veleroCLI2Version,
			},
		)()
	}
}

func MigrationWithFS() {
	for _, veleroCLI2Version := range veleroutil.GetVersionList(
		test.VeleroCfg.MigrateFromVeleroCLI,
		test.VeleroCfg.MigrateFromVeleroVersion) {
		framework.TestFunc(
			&migrationE2E{
				useVolumeSnapshots: false,
				veleroCLI2Version:  veleroCLI2Version,
			},
		)()
	}
}

func (m *migrationE2E) Init() error {
	By("Call the base E2E init", func() {
		Expect(m.TestCase.Init()).To(Succeed())
	})

	By("Skip check", func() {
		if m.VeleroCfg.DefaultClusterContext == "" || m.VeleroCfg.StandbyClusterContext == "" {
			Skip("Migration test needs 2 clusters")
		}

		if m.useVolumeSnapshots && m.VeleroCfg.CloudProvider == test.Kind {
			Skip(fmt.Sprintf("Volume snapshots not supported on %s", test.Kind))
		}

		if m.VeleroCfg.SnapshotMoveData && !m.useVolumeSnapshots {
			Skip("FSB migration test is not needed in data mover scenario")
		}
	})

	m.kibishiiData = *kibishii.DefaultKibishiiData
	m.kibishiiData.ExpectedNodes = 3
	m.CaseBaseName = "migration-" + m.UUIDgen
	m.BackupName = m.CaseBaseName + "-backup"
	m.RestoreName = m.CaseBaseName + "-restore"
	m.NSIncluded = &[]string{m.CaseBaseName}

	m.RestoreArgs = []string{
		"create", "--namespace", m.VeleroCfg.VeleroNamespace,
		"restore", m.RestoreName,
		"--from-backup", m.BackupName, "--wait",
	}

	// Message output by ginkgo
	m.TestMsg = &framework.TestMSG{
		Desc:      "Test migration workload on two clusters",
		FailedMSG: "Fail to test migrate between two clusters",
		Text:      "Test back up on default cluster, restore on standby cluster",
	}

	// Need to uninstall Velero on the default cluster.
	if test.InstallVelero {
		ctx, ctxCancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer ctxCancel()
		Expect(veleroutil.VeleroUninstall(ctx, m.VeleroCfg)).To(Succeed())
	}

	return nil
}

func (m *migrationE2E) Backup() error {
	OriginVeleroCfg := m.VeleroCfg
	var err error

	if m.veleroCLI2Version.VeleroCLI == "" {
		//Assume tag of velero server image is identical to velero CLI version
		//Download velero CLI if it's empty according to velero CLI version
		By(
			fmt.Sprintf("Install the expected version Velero CLI %s",
				m.veleroCLI2Version.VeleroVersion),
			func() {
				// "self" represents 1.14.x and future versions
				if m.veleroCLI2Version.VeleroVersion == "self" {
					m.veleroCLI2Version.VeleroCLI = m.VeleroCfg.VeleroCLI
				} else {
					OriginVeleroCfg, err = veleroutil.SetImagesToDefaultValues(
						OriginVeleroCfg,
						m.veleroCLI2Version.VeleroVersion,
					)
					Expect(err).To(Succeed(),
						"Fail to set images for the migrate-from Velero installation.")

					m.veleroCLI2Version.VeleroCLI, err = veleroutil.InstallVeleroCLI(
						m.veleroCLI2Version.VeleroVersion)
					Expect(err).To(Succeed())
				}
			},
		)
	}

	By(fmt.Sprintf("Install Velero on default cluster (%s)", m.VeleroCfg.DefaultClusterContext),
		func() {
			Expect(k8sutil.KubectlConfigUseContext(
				m.Ctx, m.VeleroCfg.DefaultClusterContext)).To(Succeed())
			OriginVeleroCfg.MigrateFromVeleroVersion = m.veleroCLI2Version.VeleroVersion
			OriginVeleroCfg.VeleroCLI = m.veleroCLI2Version.VeleroCLI
			OriginVeleroCfg.ClientToInstallVelero = OriginVeleroCfg.DefaultClient
			OriginVeleroCfg.ClusterToInstallVelero = m.VeleroCfg.DefaultClusterName
			OriginVeleroCfg.ServiceAccountNameToInstall = m.VeleroCfg.DefaultCLSServiceAccountName
			OriginVeleroCfg.UseVolumeSnapshots = m.useVolumeSnapshots
			OriginVeleroCfg.UseNodeAgent = !m.useVolumeSnapshots

			version, err := veleroutil.GetVeleroVersion(m.Ctx, OriginVeleroCfg.VeleroCLI, true)
			Expect(err).To(Succeed(), "Fail to get Velero version")
			OriginVeleroCfg.VeleroVersion = version

			if OriginVeleroCfg.SnapshotMoveData {
				OriginVeleroCfg.UseNodeAgent = true
			}

			Expect(veleroutil.VeleroInstall(m.Ctx, &OriginVeleroCfg, false)).To(Succeed())
			if m.veleroCLI2Version.VeleroVersion != "self" {
				Expect(veleroutil.CheckVeleroVersion(
					m.Ctx,
					OriginVeleroCfg.VeleroCLI,
					OriginVeleroCfg.MigrateFromVeleroVersion,
				)).To(Succeed())
			}
		},
	)

	By("Create namespace for sample workload", func() {
		Expect(k8sutil.CreateNamespace(
			m.Ctx,
			*m.VeleroCfg.DefaultClient,
			m.CaseBaseName,
		)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s to install Kibishii workload",
				m.CaseBaseName))
	})

	By("Deploy sample workload of Kibishii", func() {
		Expect(kibishii.KibishiiPrepareBeforeBackup(
			m.Ctx,
			*OriginVeleroCfg.DefaultClient,
			OriginVeleroCfg.CloudProvider,
			m.CaseBaseName,
			OriginVeleroCfg.RegistryCredentialFile,
			OriginVeleroCfg.Features,
			OriginVeleroCfg.KibishiiDirectory,
			OriginVeleroCfg.UseVolumeSnapshots,
			&m.kibishiiData,
		)).To(Succeed())
	})

	By(fmt.Sprintf("Backup namespace %s", m.CaseBaseName), func() {
		m.BackupArgs = []string{
			"create", "--namespace", m.VeleroCfg.VeleroNamespace,
			"backup", m.BackupName,
			"--include-namespaces", strings.Join(*m.NSIncluded, ","),
			"--wait",
		}

		if m.useVolumeSnapshots {
			m.BackupArgs = append(m.BackupArgs, "--snapshot-volumes=true")
		} else {
			m.BackupArgs = append(m.BackupArgs, "--default-volumes-to-fs-backup")
		}

		if OriginVeleroCfg.SnapshotMoveData {
			m.BackupArgs = append(m.BackupArgs, "--snapshot-move-data")
		}

		Expect(veleroutil.VeleroBackupExec(
			m.Ctx,
			OriginVeleroCfg.VeleroCLI,
			OriginVeleroCfg.VeleroNamespace,
			m.BackupName,
			m.BackupArgs,
		)).To(Succeed(), func() string {
			veleroutil.RunDebug(
				context.Background(),
				OriginVeleroCfg.VeleroCLI,
				OriginVeleroCfg.VeleroNamespace,
				m.BackupName,
				"",
			)
			return "Failed to backup resources"
		})
	})

	if m.useVolumeSnapshots {
		// Only wait for the snapshots.backupdriver.cnsdp.vmware.com
		// when the vSphere plugin is used.
		if OriginVeleroCfg.HasVspherePlugin {
			By("Waiting for vSphere uploads to complete", func() {
				Expect(
					veleroutil.WaitForVSphereUploadCompletion(
						context.Background(),
						time.Hour,
						m.CaseBaseName,
						m.kibishiiData.ExpectedNodes,
					),
				).To(Succeed())
			})
		}

		var snapshotCheckPoint test.SnapshotCheckPoint
		snapshotCheckPoint.NamespaceBackedUp = m.CaseBaseName

		if OriginVeleroCfg.SnapshotMoveData {
			//VolumeSnapshotContent should be deleted after data movement
			_, err := util.CheckVolumeSnapshotCR(
				*m.VeleroCfg.DefaultClient,
				map[string]string{"namespace": m.CaseBaseName},
				0,
			)
			By("Check the VSC account", func() {
				Expect(err).NotTo(HaveOccurred(), "VSC count is not as expected 0")
			})
		} else {
			// the snapshots of AWS may be still in pending status when do the restore.
			// wait for a while to avoid this https://github.com/vmware-tanzu/velero/issues/1799
			if OriginVeleroCfg.CloudProvider == test.Azure &&
				strings.EqualFold(OriginVeleroCfg.Features, test.FeatureCSI) ||
				OriginVeleroCfg.CloudProvider == test.AWS {
				By("Sleep 5 minutes to avoid snapshot recreated by unknown reason ", func() {
					time.Sleep(5 * time.Minute)
				})
			}

			By("Snapshot should be created in cloud object store with retain policy", func() {
				snapshotCheckPoint, err = veleroutil.GetSnapshotCheckPoint(
					*OriginVeleroCfg.DefaultClient,
					OriginVeleroCfg,
					m.kibishiiData.ExpectedNodes,
					m.CaseBaseName,
					m.BackupName,
					kibishii.GetKibishiiPVCNameList(m.kibishiiData.ExpectedNodes),
				)

				Expect(err).NotTo(HaveOccurred(), "Fail to get snapshot checkpoint")
				Expect(providers.CheckSnapshotsInProvider(
					OriginVeleroCfg,
					m.BackupName,
					snapshotCheckPoint,
					false,
				)).To(Succeed())
			})
		}
	}

	return nil
}

func (m *migrationE2E) Restore() error {
	StandbyVeleroCfg := m.VeleroCfg

	By("Install Velero in standby cluster.", func() {
		// Ensure cluster-B is the target cluster
		Expect(k8sutil.KubectlConfigUseContext(
			m.Ctx, m.VeleroCfg.StandbyClusterContext)).To(Succeed())

		// Check the workload namespace not exist in standby cluster.
		_, err := k8sutil.GetNamespace(
			m.Ctx, *m.VeleroCfg.StandbyClient, m.CaseBaseName)
		Expect(err).To(HaveOccurred(), fmt.Sprintf(
			"get namespace in dst cluster successfully, it's not as expected: %s", m.CaseBaseName))
		Expect(strings.Contains(fmt.Sprint(err), "namespaces \""+m.CaseBaseName+"\" not found")).
			Should(BeTrue())

		By("Install StorageClass for E2E.")
		Expect(veleroutil.InstallStorageClasses(
			m.VeleroCfg.StandbyClusterCloudProvider)).To(Succeed())

		if strings.EqualFold(m.VeleroCfg.Features, test.FeatureCSI) &&
			m.VeleroCfg.UseVolumeSnapshots {
			By("Install VolumeSnapshotClass for E2E.")
			Expect(
				k8sutil.KubectlApplyByFile(
					m.Ctx,
					fmt.Sprintf("../testdata/volume-snapshot-class/%s.yaml",
						m.VeleroCfg.StandbyClusterCloudProvider),
				),
			).To(Succeed())
		}

		StandbyVeleroCfg.ClientToInstallVelero = m.VeleroCfg.StandbyClient
		StandbyVeleroCfg.ClusterToInstallVelero = m.VeleroCfg.StandbyClusterName
		StandbyVeleroCfg.ServiceAccountNameToInstall = m.VeleroCfg.StandbyCLSServiceAccountName
		StandbyVeleroCfg.UseNodeAgent = !m.useVolumeSnapshots
		if StandbyVeleroCfg.SnapshotMoveData {
			StandbyVeleroCfg.UseNodeAgent = true
			// For SnapshotMoveData pipelines, we should use standby cluster setting
			// for Velero installation.
			// In nightly CI, StandbyClusterPlugins is set properly
			// if pipeline is for SnapshotMoveData.
			StandbyVeleroCfg.Plugins = m.VeleroCfg.StandbyClusterPlugins
			StandbyVeleroCfg.ObjectStoreProvider = m.VeleroCfg.StandbyClusterObjectStoreProvider
		}

		Expect(veleroutil.VeleroInstall(
			context.Background(), &StandbyVeleroCfg, true)).To(Succeed())
	})

	By("Waiting for backups sync to Velero in standby cluster", func() {
		Expect(veleroutil.WaitForBackupToBeCreated(
			m.Ctx, m.BackupName, 5*time.Minute, &StandbyVeleroCfg)).To(Succeed())
	})

	By(fmt.Sprintf("Restore %s", m.CaseBaseName), func() {
		cmName := "datamover-storage-class-config"
		labels := map[string]string{"velero.io/change-storage-class": "RestoreItemAction",
			"velero.io/plugin-config": ""}
		data := map[string]string{kibishii.KibishiiStorageClassName: test.StorageClassName}

		By(fmt.Sprintf("Create ConfigMap %s in namespace %s",
			cmName, StandbyVeleroCfg.VeleroNamespace), func() {
			_, err := k8sutil.CreateConfigMap(
				StandbyVeleroCfg.StandbyClient.ClientGo,
				StandbyVeleroCfg.VeleroNamespace,
				cmName,
				labels,
				data,
			)
			Expect(err).To(Succeed(), fmt.Sprintf(
				"failed to create ConfigMap in the namespace %q",
				StandbyVeleroCfg.VeleroNamespace))
		})

		Expect(veleroutil.VeleroRestore(
			m.Ctx,
			StandbyVeleroCfg.VeleroCLI,
			StandbyVeleroCfg.VeleroNamespace,
			m.RestoreName,
			m.BackupName,
			"",
		)).To(Succeed(), func() string {
			veleroutil.RunDebug(
				m.Ctx, StandbyVeleroCfg.VeleroCLI,
				StandbyVeleroCfg.VeleroNamespace, "", m.RestoreName)
			return "Fail to restore workload"
		})
	})

	return nil
}

func (m *migrationE2E) Verify() error {
	By(fmt.Sprintf("Verify workload %s after restore on standby cluster", m.CaseBaseName), func() {
		Expect(kibishii.KibishiiVerifyAfterRestore(
			*m.VeleroCfg.StandbyClient,
			m.CaseBaseName,
			m.Ctx,
			&m.kibishiiData,
			"",
		)).To(Succeed(), "Fail to verify workload after restore")
	})

	return nil
}

func (m *migrationE2E) Clean() error {
	By("Clean resource on default cluster.", func() {
		Expect(m.TestCase.Clean()).To(Succeed())
	})

	By("Clean resource on standby cluster.", func() {
		Expect(k8sutil.KubectlConfigUseContext(
			m.Ctx, m.VeleroCfg.StandbyClusterContext)).To(Succeed())
		m.VeleroCfg.ClientToInstallVelero = m.VeleroCfg.StandbyClient
		m.VeleroCfg.ClusterToInstallVelero = m.VeleroCfg.StandbyClusterName

		By("Delete StorageClasses created by E2E")
		Expect(
			k8sutil.DeleteStorageClass(
				m.Ctx,
				*m.VeleroCfg.ClientToInstallVelero,
				test.StorageClassName,
			),
		).To(Succeed())
		Expect(
			k8sutil.DeleteStorageClass(
				m.Ctx,
				*m.VeleroCfg.ClientToInstallVelero,
				test.StorageClassName2,
			),
		).To(Succeed())

		if strings.EqualFold(m.VeleroCfg.Features, test.FeatureCSI) &&
			m.VeleroCfg.UseVolumeSnapshots {
			By("Delete VolumeSnapshotClass created by E2E")
			Expect(
				k8sutil.KubectlDeleteByFile(
					m.Ctx,
					fmt.Sprintf("../testdata/volume-snapshot-class/%s.yaml",
						m.VeleroCfg.StandbyClusterCloudProvider),
				),
			).To(Succeed())
		}

		Expect(veleroutil.VeleroUninstall(m.Ctx, m.VeleroCfg)).To(Succeed())

		Expect(
			k8sutil.DeleteNamespace(
				m.Ctx,
				*m.VeleroCfg.StandbyClient,
				m.CaseBaseName,
				true,
			),
		).To(Succeed())
	})

	By("Switch to default KubeConfig context", func() {
		Expect(k8sutil.KubectlConfigUseContext(
			m.Ctx,
			m.VeleroCfg.DefaultClusterContext,
		)).To(Succeed())
	})

	return nil
}
