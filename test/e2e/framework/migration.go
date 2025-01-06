package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/test"
	velerotest "github.com/vmware-tanzu/velero/test"
	k8sutil "github.com/vmware-tanzu/velero/test/util/k8s"
	"github.com/vmware-tanzu/velero/test/util/kibishii"
	veleroutil "github.com/vmware-tanzu/velero/test/util/velero"
)

/*
MigrationTest interface is specific for the migration test scenario.
*/
type MigrationTest interface {
	Init() error
	InstallVeleroOnActiveCluster() error
	InstallVeleroOnStandbyCluster() error
	CreateResourcesOnActiveCluster() error
	BackupOnActiveCluster() error
	RestoreOnStandbyCluster() error
	VerifyOnStandbyCluster() error
	CleanOnBothCluster() error
	GetTestMsg() *TestMSG
}

type MigrationCase struct {
	BackupName         string
	RestoreName        string
	CaseBaseName       string
	BackupArgs         []string
	RestoreArgs        []string
	TestMsg            *TestMSG
	UseVolumeSnapshots bool
	VeleroCfg          velerotest.VeleroConfig
	Ctx                context.Context
	CtxCancel          context.CancelFunc
	UUIDgen            string
	VeleroCLI2Version  test.VeleroCLI2Version
	KibishiiData       kibishii.KibishiiData
}

func (m *MigrationCase) Init() error {
	m.Ctx, m.CtxCancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer m.CtxCancel()

	m.UUIDgen = GenerateUUID()
	m.VeleroCfg = velerotest.VeleroCfg

	m.CaseBaseName = "migration-" + m.UUIDgen

	m.KibishiiData = *kibishii.DefaultKibishiiData
	m.KibishiiData.ExpectedNodes = 3

	// Message output by ginkgo
	m.TestMsg = &TestMSG{
		Desc:      "Test migration workload on two clusters",
		FailedMSG: "Fail to test migrate between two clusters",
		Text:      "Test back up on default cluster, restore on standby cluster",
	}

	return nil
}

func (m *MigrationCase) InstallVeleroOnActiveCluster() error {
	if m.VeleroCLI2Version.VeleroCLI == "" {
		//Assume tag of velero server image is identical to velero CLI version
		//Download velero CLI if it's empty according to velero CLI version
		By(
			fmt.Sprintf("Install the expected version Velero CLI %s",
				m.VeleroCLI2Version.VeleroVersion),
			func() {
				var err error
				// "self" represents 1.14.x and future versions
				if m.VeleroCLI2Version.VeleroVersion == "self" {
					m.VeleroCLI2Version.VeleroCLI = m.VeleroCfg.VeleroCLI
				} else {
					m.VeleroCfg, err = veleroutil.SetImagesToDefaultValues( // TODO
						m.VeleroCfg,
						m.VeleroCLI2Version.VeleroVersion,
					)
					Expect(err).To(Succeed(),
						"Fail to set images for the migrate-from Velero installation.")

					m.VeleroCLI2Version.VeleroCLI, err = veleroutil.InstallVeleroCLI(
						m.VeleroCLI2Version.VeleroVersion)
					Expect(err).To(Succeed())
				}
			},
		)
	}

	By(fmt.Sprintf("Install Velero on default cluster (%s)", m.VeleroCfg.ActiveClusterContext),
		func() {
			Expect(k8sutil.KubectlConfigUseContext(
				m.Ctx, m.VeleroCfg.ActiveClusterContext)).To(Succeed())
			m.VeleroCfg.MigrateFromVeleroVersion = m.VeleroCLI2Version.VeleroVersion
			m.VeleroCfg.VeleroCLI = m.VeleroCLI2Version.VeleroCLI
			m.VeleroCfg.ClientToInstallVelero = m.VeleroCfg.ActiveClient
			m.VeleroCfg.ClusterToInstallVelero = m.VeleroCfg.ActiveClusterName
			m.VeleroCfg.ServiceAccountNameToInstall = m.VeleroCfg.ActiveCLSServiceAccountName
			m.VeleroCfg.UseVolumeSnapshots = m.UseVolumeSnapshots
			m.VeleroCfg.UseNodeAgent = !m.UseVolumeSnapshots

			version, err := veleroutil.GetVeleroVersion(m.Ctx, m.VeleroCLI2Version.VeleroVersion, true)
			Expect(err).To(Succeed(), "Fail to get Velero version")
			m.VeleroCfg.VeleroVersion = version

			Expect(veleroutil.VeleroInstall(m.Ctx, &m.VeleroCfg, false)).To(Succeed())
			if m.VeleroCLI2Version.VeleroVersion != "self" {
				Expect(veleroutil.CheckVeleroVersion(
					m.Ctx,
					m.VeleroCfg.VeleroCLI,                // TODO
					m.VeleroCfg.MigrateFromVeleroVersion, // TODO
				)).To(Succeed())
			}
		},
	)

	return nil
}

func (m *MigrationCase) InstallVeleroOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) CreateResourcesOnActiveCluster() error {
	By("Create namespace for sample workload", func() {
		Expect(k8sutil.CreateNamespace(
			m.Ctx,
			*m.VeleroCfg.ActiveClient,
			m.CaseBaseName,
		)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s to install Kibishii workload",
				m.CaseBaseName))
	})

	By("Deploy sample workload of Kibishii", func() {
		Expect(kibishii.KibishiiPrepareBeforeBackup(
			m.Ctx,
			*m.VeleroCfg.ActiveClient,
			m.VeleroCfg.CloudProvider,
			m.CaseBaseName,
			m.VeleroCfg.RegistryCredentialFile,
			m.VeleroCfg.Features,
			m.VeleroCfg.KibishiiDirectory,
			m.VeleroCfg.UseVolumeSnapshots,
			&m.KibishiiData,
		)).To(Succeed())
	})

	return nil
}

func (m *MigrationCase) BackupOnActiveCluster() error {
	By(fmt.Sprintf("Backup namespace %s", m.CaseBaseName), func() {
		m.BackupArgs = []string{
			"create", "--namespace", m.VeleroCfg.VeleroNamespace,
			"backup", m.BackupName,
			"--include-namespaces", m.CaseBaseName,
			"--wait",
		}

		if m.UseVolumeSnapshots {
			m.BackupArgs = append(m.BackupArgs, "--snapshot-volumes=true")
		} else {
			m.BackupArgs = append(m.BackupArgs, "--default-volumes-to-fs-backup")
		}

		if m.VeleroCfg.SnapshotMoveData {
			m.BackupArgs = append(m.BackupArgs, "--snapshot-move-data")
		}

		Expect(veleroutil.VeleroBackupExec(
			m.Ctx,
			m.VeleroCfg.VeleroCLI, // TODO
			m.VeleroCfg.VeleroNamespace,
			m.BackupName,
			m.BackupArgs,
		)).To(Succeed(), func() string {
			veleroutil.RunDebug(
				context.Background(),
				m.VeleroCfg.VeleroCLI, // TODO
				m.VeleroCfg.VeleroNamespace,
				m.BackupName,
				"",
			)
			return "Failed to backup resources"
		})
	})

	return nil
}

func (m *MigrationCase) RestoreOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) VerifyOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) CleanOnBothCluster() error {
	By("Clean resource on active cluster.", func() {
		Expect(veleroutil.DeleteVeleroAndRelatedResources(
			m.Ctx,
			m.VeleroCfg.ActiveClusterContext,
			m.VeleroCfg.ActiveClient,
			m.VeleroCfg.ProviderName,
		)).To(Succeed())

		Expect(
			k8sutil.DeleteNamespace(
				m.Ctx,
				*m.VeleroCfg.ActiveClient,
				m.CaseBaseName,
				true,
			),
		).To(Succeed())
	})

	By("Clean resource on standby cluster.", func() {
		Expect(veleroutil.DeleteVeleroAndRelatedResources(
			m.Ctx,
			m.VeleroCfg.StandbyClusterContext,
			m.VeleroCfg.StandbyClient,
			m.VeleroCfg.StandbyClusterCloudProvider,
		)).To(Succeed())

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
			m.VeleroCfg.ActiveClusterContext,
		)).To(Succeed())
	})

	return nil
}

func (m *MigrationCase) GetTestMsg() *TestMSG {
	return m.TestMsg
}

func MigrationTestFunc(test MigrationTest) {
	It("Run migration E2E test case", func() {
		Expect(RunMigrationTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
	})
}

func RunMigrationTestCase(test MigrationTest) error {
	if test == nil {
		return errors.New("No case should be tested")
	}

	if err := test.Init(); err != nil {
		fmt.Println("Fail to init the migration test: ", err)
		return err
	}

	if err := test.InstallVeleroOnActiveCluster(); err != nil {
		fmt.Println("Fail to install Velero on active cluster: ", err)
		return err
	}

	if err := test.InstallVeleroOnStandbyCluster(); err != nil {
		fmt.Println("Fail to install Velero on standby cluster: ", err)
		return err
	}

	if err := test.CreateResourcesOnActiveCluster(); err != nil {
		fmt.Println("Fail to create resources on active cluster: ", err)
		return err
	}

	if err := test.BackupOnActiveCluster(); err != nil {
		fmt.Println("Fail to run backup on active cluster: ", err)
		return err
	}

	if err := test.RestoreOnStandbyCluster(); err != nil {
		fmt.Println("Fail to run restore on standby cluster: ", err)
		return err
	}

	if err := test.VerifyOnStandbyCluster(); err != nil {
		fmt.Println("Fail to verify the restore result on standby cluster: ", err)
		return err
	}

	if err := test.CleanOnBothCluster(); err != nil {
		fmt.Println("Fail to clean the resources on both clusters: ", err)
		return err
	}

	return nil
}
