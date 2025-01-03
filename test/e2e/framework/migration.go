package framework

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerotest "github.com/vmware-tanzu/velero/test"
	k8sutil "github.com/vmware-tanzu/velero/test/util/k8s"
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
	GetTestCase() *MigrationCase
}

type MigrationCase struct {
	BackupName         string
	RestoreName        string
	CaseBaseName       string
	BackupArgs         []string
	RestoreArgs        []string
	NamespacesTotal    int
	TestMsg            *TestMSG
	Client             k8sutil.TestClient
	NSIncluded         *[]string
	UseVolumeSnapshots bool
	VeleroCfg          velerotest.VeleroConfig
	RestorePhaseExpect velerov1api.RestorePhase
	Ctx                context.Context
	CtxCancel          context.CancelFunc
	UUIDgen            string
}

func (m *MigrationCase) Init() error {
	return nil
}

func (m *MigrationCase) InstallVeleroOnActiveCluster() error {
	return nil
}

func (m *MigrationCase) InstallVeleroOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) CreateResourcesOnActiveCluster() error {
	return nil
}

func (m *MigrationCase) BackupOnActiveCluster() error {
	return nil
}

func (m *MigrationCase) RestoreOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) VerifyOnStandbyCluster() error {
	return nil
}

func (m *MigrationCase) CleanOnBothCluster() error {
	return nil
}

func (m *MigrationCase) GetTestMsg() *TestMSG {
	return nil
}

func MigrationTestFunc(test MigrationTest) func() {
	return func() {
		TestMigrationIt(test)
	}
}

func TestMigrationIt(test MigrationTest) {
	It("Run E2E test case", func() {
		Expect(test.Init()).To(Succeed())

		Expect(RunMigrationTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
	})
}

func RunMigrationTestCase(test MigrationTest) error {
	if test == nil {
		return errors.New("No case should be tested")
	}
	return nil
}
