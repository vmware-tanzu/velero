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

package test

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

/*
The VeleroBackupRestoreTest interface is just could be suit for the cases that follow the test flow of
create resources, backup, delete test resource, restore and verify.
And the cases have similar execute function and similar data. it's both fine for you to use it or not which
depends on your test patterns.
*/
type VeleroBackupRestoreTest interface {
	Init() error
	CreateResources() error
	Backup() error
	Destroy() error
	Restore() error
	Verify() error
	Clean() error
	GetTestMsg() *TestMSG
	GetTestCase() *TestCase
}

type TestMSG struct {
	Desc      string
	Text      string
	FailedMSG string
}

type TestCase struct {
	BackupName         string
	RestoreName        string
	CaseBaseName       string
	BackupArgs         []string
	RestoreArgs        []string
	NamespacesTotal    int
	TestMsg            *TestMSG
	Client             TestClient
	NSIncluded         *[]string
	UseVolumeSnapshots bool
	VeleroCfg          VeleroConfig
	RestorePhaseExpect velerov1api.RestorePhase
	Ctx                context.Context
	CtxCancel          context.CancelFunc
	UUIDgen            string
}

func TestFunc(test VeleroBackupRestoreTest) func() {
	return func() {
		Expect(test.Init()).To(Succeed(), "Failed to instantiate test cases")
		BeforeEach(func() {
			flag.Parse()
			// Using the global velero config which covered the installation for most common cases
			veleroCfg := e2e.VeleroCfg
			// TODO: Skip nodeport test until issue https://github.com/kubernetes/kubernetes/issues/114384 fixed
			// TODO: Although this issue is closed, but it's not fixed.
			// TODO: After bump up k8s version in AWS pipeline, this issue also apply for AWS pipeline.
			if (veleroCfg.CloudProvider == "azure" || veleroCfg.CloudProvider == "aws") && strings.Contains(test.GetTestCase().CaseBaseName, "nodeport") {
				Skip("Skip due to issue https://github.com/kubernetes/kubernetes/issues/114384 on AKS")
			}
			if veleroCfg.InstallVelero {
				Expect(PrepareVelero(context.Background(), test.GetTestCase().CaseBaseName)).To(Succeed())
			}
		})
		It(test.GetTestMsg().Text, func() {
			Expect(RunTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
		})
	}
}

func TestFuncWithMultiIt(tests []VeleroBackupRestoreTest) func() {
	return func() {
		veleroCfg := e2e.VeleroCfg
		for k := range tests {
			Expect(tests[k].Init()).To(Succeed(), fmt.Sprintf("Failed to instantiate test %s case", tests[k].GetTestMsg().Desc))
			defer tests[k].GetTestCase().CtxCancel()
		}

		BeforeEach(func() {
			flag.Parse()
			if veleroCfg.InstallVelero {
				Expect(PrepareVelero(context.Background(), tests[0].GetTestCase().CaseBaseName)).To(Succeed())
			}
		})

		for k := range tests {
			curTest := tests[k]
			It(curTest.GetTestMsg().Text, func() {
				Expect(RunTestCase(curTest)).To(Succeed(), curTest.GetTestMsg().FailedMSG)
			})
		}
	}
}

func (t *TestCase) Init() error {
	t.Ctx, t.CtxCancel = context.WithTimeout(context.Background(), 1*time.Hour)
	t.UUIDgen = t.GenerateUUID()
	return nil
}

func (t *TestCase) GenerateUUID() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%08d", rand.Intn(100000000))
}

func (t *TestCase) CreateResources() error {
	return nil
}

func (t *TestCase) Backup() error {
	veleroCfg := t.GetTestCase().VeleroCfg
	if err := VeleroBackupExec(t.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.BackupName, t.BackupArgs); err != nil {
		RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.BackupName, "")
		return errors.Wrapf(err, "Failed to backup resources")
	}
	return nil
}

func (t *TestCase) Destroy() error {
	By(fmt.Sprintf("Start to destroy namespace %s......", t.CaseBaseName), func() {
		Expect(CleanupNamespacesWithPoll(t.Ctx, t.Client, t.CaseBaseName)).To(Succeed(), "Could cleanup retrieve namespaces")
	})
	return nil
}

func (t *TestCase) Restore() error {
	if len(t.RestoreArgs) == 0 {
		return nil
	}

	veleroCfg := t.GetTestCase().VeleroCfg
	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	if t.UseVolumeSnapshots && veleroCfg.CloudProvider != "vsphere" {
		fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
		time.Sleep(5 * time.Minute)
	}

	By("Start to restore ......", func() {
		if t.RestorePhaseExpect == "" {
			t.RestorePhaseExpect = velerov1api.RestorePhaseCompleted
		}
		Expect(VeleroRestoreExec(t.Ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, t.RestoreName, t.RestoreArgs, t.RestorePhaseExpect)).To(Succeed(), func() string {
			RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, "", t.RestoreName)
			return "Fail to restore workload"
		})
	})
	return nil
}

func (t *TestCase) Verify() error {
	return nil
}

func (t *TestCase) Clean() error {
	veleroCfg := t.GetTestCase().VeleroCfg
	if !veleroCfg.Debug {
		By(fmt.Sprintf("Clean namespace with prefix %s after test", t.CaseBaseName), func() {
			CleanupNamespaces(t.Ctx, t.Client, t.CaseBaseName)
		})
		By("Clean backups after test", func() {
			DeleteBackups(t.Ctx, t.Client)
		})
	}
	return nil
}

func (t *TestCase) GetTestMsg() *TestMSG {
	return t.TestMsg
}

func (t *TestCase) GetTestCase() *TestCase {
	return t
}

func RunTestCase(test VeleroBackupRestoreTest) error {
	fmt.Printf("Running test case %s %s\n", test.GetTestMsg().Desc, time.Now().Format("2006-01-02 15:04:05"))
	if test == nil {
		return errors.New("No case should be tested")
	}
	defer test.Clean()
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
	err = test.Destroy()
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
