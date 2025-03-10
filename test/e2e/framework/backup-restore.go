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

package framework

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerotest "github.com/vmware-tanzu/velero/test"
	k8sutil "github.com/vmware-tanzu/velero/test/util/k8s"
	veleroutil "github.com/vmware-tanzu/velero/test/util/velero"
)

/*
The BackupRestoreTest interface is just could be suit for the cases that follow the test flow of
create resources, backup, delete test resource, restore and verify.
And the cases have similar execute function and similar data.
*/
type BackupRestoreTest interface {
	Init() error
	InstallVelero() error
	CreateResources() error
	Backup() error
	DeleteResources() error
	Restore() error
	Verify() error
	Clean() error
	GetTestMsg() *TestMSG
}

type TestMSG struct {
	Desc      string
	Text      string
	FailedMSG string
}

type BRCase struct {
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

func TestFunc(test BackupRestoreTest) func() {
	return func() {
		TestIt(test)
	}
}

func TestFuncWithMultiIt(tests []BackupRestoreTest) func() {
	return func() {
		for k := range tests {
			TestIt(tests[k])
		}
	}
}

func TestIt(test BackupRestoreTest) {
	It("Run BackupRestore E2E test case", func() {
		Expect(RunTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
	})
}

func (br *BRCase) Init() error {
	br.Ctx, br.CtxCancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer br.CtxCancel()

	br.UUIDgen = GenerateUUID()
	br.VeleroCfg = velerotest.VeleroCfg
	br.Client = *br.VeleroCfg.ClientToInstallVelero
	return nil
}

func (br *BRCase) InstallVelero() error {
	if velerotest.InstallVelero {
		return veleroutil.PrepareVelero(br.Ctx, br.CaseBaseName, br.VeleroCfg)
	}

	return nil
}

func (br *BRCase) CreateResources() error {
	return nil
}

func (br *BRCase) Backup() error {
	By("Start to backup ......", func() {
		br.BackupName = "backup-" + br.CaseBaseName

		Expect(
			veleroutil.VeleroBackupExec(
				br.Ctx,
				br.VeleroCfg.VeleroCLI,
				br.VeleroCfg.VeleroNamespace,
				br.BackupName,
				br.BackupArgs,
			)).To(
			Succeed(),
			func() string {
				veleroutil.RunDebug(
					br.Ctx,
					br.VeleroCfg.VeleroCLI,
					br.VeleroCfg.VeleroNamespace,
					br.BackupName,
					"",
				)

				return "Failed to backup resources"
			})
	})

	return nil
}

func (br *BRCase) DeleteResources() error {
	By(fmt.Sprintf("Start to delete namespace %s......", br.CaseBaseName), func() {
		Expect(k8sutil.CleanupNamespacesWithPoll(br.Ctx, br.Client, br.CaseBaseName)).To(Succeed(), "Could cleanup retrieve namespaces")
	})
	return nil
}

func (br *BRCase) Restore() error {
	if len(br.RestoreArgs) == 0 {
		return nil
	}

	br.RestoreName = "restore-" + br.CaseBaseName

	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	By("Waiting 5 minutes to make sure the snapshots are ready...", func() {
		if br.UseVolumeSnapshots && br.VeleroCfg.CloudProvider != velerotest.Vsphere {
			time.Sleep(5 * time.Minute)
		}
	})

	By("Start to restore ......", func() {
		if br.RestorePhaseExpect == "" {
			br.RestorePhaseExpect = velerov1api.RestorePhaseCompleted
		}
		Expect(
			veleroutil.VeleroRestoreExec(
				br.Ctx,
				br.VeleroCfg.VeleroCLI,
				br.VeleroCfg.VeleroNamespace,
				br.RestoreName,
				br.RestoreArgs,
				br.RestorePhaseExpect,
			)).To(
			Succeed(),
			func() string {
				veleroutil.RunDebug(
					br.Ctx,
					br.VeleroCfg.VeleroCLI,
					br.VeleroCfg.VeleroNamespace,
					"",
					br.RestoreName,
				)

				return "Fail to restore workload"
			})
	})
	return nil
}

func (br *BRCase) Verify() error {
	return nil
}

func (br *BRCase) Clean() (err error) {
	veleroCfg := br.VeleroCfg
	if CurrentSpecReport().Failed() && veleroCfg.FailFast {
		fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
	} else {
		By(fmt.Sprintf("Clean namespace with prefix %s after test", br.CaseBaseName), func() {
			if err = k8sutil.CleanupNamespaces(br.Ctx, br.Client, br.CaseBaseName); err != nil {
				fmt.Println("Fail to cleanup namespaces: ", err)
			}
		})
		By("Clean backups after test", func() {
			veleroCfg.ClientToInstallVelero = &br.Client
			if err = veleroutil.DeleteAllBackups(br.Ctx, &veleroCfg); err != nil {
				fmt.Println("Fail to clean backups after test: ", err)
			}
		})
	}

	return err
}

func (br *BRCase) GetTestMsg() *TestMSG {
	return br.TestMsg
}

func RunTestCase(test BackupRestoreTest) error {
	if test == nil {
		return errors.New("No case should be tested")
	}

	fmt.Println("Start to run case: ", test.GetTestMsg().Text)
	Expect(test.Init()).To(Succeed())

	defer Expect(test.Clean()).To(Succeed())

	fmt.Println("Install Velero: ", test.GetTestMsg().Text)
	Expect(test.InstallVelero()).To(Succeed())

	fmt.Printf("Running test case %s %s\n", test.GetTestMsg().Desc, time.Now().Format("2006-01-02 15:04:05"))

	fmt.Printf("CreateResources %s\n", time.Now().Format("2006-01-02 15:04:05"))
	err := test.CreateResources()
	if err != nil {
		return err
	}
	fmt.Printf("Backup %s\n", time.Now().Format("2006-01-02 15:04:05"))
	err = test.Backup()
	if err != nil {
		return err
	}
	fmt.Printf("Destroy %s\n", time.Now().Format("2006-01-02 15:04:05"))
	err = test.DeleteResources()
	if err != nil {
		return err
	}
	fmt.Printf("Restore %s\n", time.Now().Format("2006-01-02 15:04:05"))
	err = test.Restore()
	if err != nil {
		return err
	}
	fmt.Printf("Verify %s\n", time.Now().Format("2006-01-02 15:04:05"))
	err = test.Verify()
	if err != nil {
		return err
	}
	fmt.Printf("Finish run test %s\n", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

func GenerateUUID() string {
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		fmt.Println("Fail to generate the random number: ", err)
		return ""
	}
	return fmt.Sprintf("%08d", n)
}
