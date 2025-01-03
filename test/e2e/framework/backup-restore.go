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
	It("Run E2E test case", func() {
		Expect(RunTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
	})
}

func (t *BRCase) Init() error {
	defer t.CtxCancel()

	t.UUIDgen = GenerateUUID()
	t.VeleroCfg = velerotest.VeleroCfg
	t.Client = *t.VeleroCfg.ClientToInstallVelero
	return nil
}

func (t *BRCase) InstallVelero() error {
	if velerotest.InstallVelero {
		return veleroutil.PrepareVelero(t.Ctx, t.CaseBaseName, t.VeleroCfg)
	}

	return nil
}

func (t *BRCase) CreateResources() error {
	return nil
}

func (t *BRCase) Backup() error {
	veleroCfg := t.VeleroCfg

	By("Start to backup ......", func() {
		Expect(veleroutil.VeleroBackupExec(t.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.BackupName, t.BackupArgs)).To(Succeed(), func() string {
			veleroutil.RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.BackupName, "")
			return "Failed to backup resources"
		})
	})

	return nil
}

func (t *BRCase) DeleteResources() error {
	By(fmt.Sprintf("Start to delete namespace %s......", t.CaseBaseName), func() {
		Expect(k8sutil.CleanupNamespacesWithPoll(t.Ctx, t.Client, t.CaseBaseName)).To(Succeed(), "Could cleanup retrieve namespaces")
	})
	return nil
}

func (t *BRCase) Restore() error {
	if len(t.RestoreArgs) == 0 {
		return nil
	}

	veleroCfg := t.VeleroCfg

	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	By("Waiting 5 minutes to make sure the snapshots are ready...", func() {
		if t.UseVolumeSnapshots && veleroCfg.CloudProvider != velerotest.Vsphere {
			time.Sleep(5 * time.Minute)
		}
	})

	By("Start to restore ......", func() {
		if t.RestorePhaseExpect == "" {
			t.RestorePhaseExpect = velerov1api.RestorePhaseCompleted
		}
		Expect(veleroutil.VeleroRestoreExec(t.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.RestoreName, t.RestoreArgs, t.RestorePhaseExpect)).To(Succeed(), func() string {
			veleroutil.RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, "", t.RestoreName)
			return "Fail to restore workload"
		})
	})
	return nil
}

func (t *BRCase) Verify() error {
	return nil
}

func (t *BRCase) Clean() (err error) {
	veleroCfg := t.VeleroCfg
	if CurrentSpecReport().Failed() && veleroCfg.FailFast {
		fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
	} else {
		By(fmt.Sprintf("Clean namespace with prefix %s after test", t.CaseBaseName), func() {
			if err = k8sutil.CleanupNamespaces(t.Ctx, t.Client, t.CaseBaseName); err != nil {
				fmt.Println("Fail to cleanup namespaces: ", err)
			}
		})
		By("Clean backups after test", func() {
			veleroCfg.ClientToInstallVelero = &t.Client
			if err = veleroutil.DeleteAllBackups(t.Ctx, &veleroCfg); err != nil {
				fmt.Println("Fail to clean backups after test: ", err)
			}
		})
	}

	return err
}

func (t *BRCase) GetTestMsg() *TestMSG {
	return t.TestMsg
}

func RunTestCase(test BackupRestoreTest) error {
	fmt.Println("Start to run case: ", test.GetTestMsg().Text)
	Expect(test.Init()).To(Succeed())

	if test == nil {
		return errors.New("No case should be tested")
	}

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
