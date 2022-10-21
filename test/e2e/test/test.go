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
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

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
	StartRun() error
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
	NSBaseName         string
	BackupArgs         []string
	RestoreArgs        []string
	NamespacesTotal    int
	TestMsg            *TestMSG
	Client             TestClient
	Ctx                context.Context
	NSIncluded         *[]string
	UseVolumeSnapshots bool
}

var TestClientInstance TestClient

func TestFunc(test VeleroBackupRestoreTest) func() {
	return func() {
		By("Create test client instance", func() {
			TestClientInstance = *VeleroCfg.ClientToInstallVelero
		})
		Expect(test.Init()).To(Succeed(), "Failed to instantiate test cases")
		BeforeEach(func() {
			flag.Parse()
			if VeleroCfg.InstallVelero {
				Expect(VeleroInstall(context.Background(), &VeleroCfg, test.GetTestCase().UseVolumeSnapshots)).To(Succeed())
			}
		})
		AfterEach(func() {
			if !VeleroCfg.Debug {
				if VeleroCfg.InstallVelero {
					Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)).To((Succeed()))
				}
			}
		})
		It(test.GetTestMsg().Text, func() {
			Expect(RunTestCase(test)).To(Succeed(), test.GetTestMsg().FailedMSG)
		})
	}
}

func TestFuncWithMultiIt(tests []VeleroBackupRestoreTest) func() {
	return func() {
		var countIt int
		By("Create test client instance", func() {
			TestClientInstance = *VeleroCfg.ClientToInstallVelero
		})
		var useVolumeSnapshots bool
		for k := range tests {
			Expect(tests[k].Init()).To(Succeed(), fmt.Sprintf("Failed to instantiate test %s case", tests[k].GetTestMsg().Desc))
			useVolumeSnapshots = tests[k].GetTestCase().UseVolumeSnapshots
		}

		BeforeEach(func() {
			flag.Parse()
			if VeleroCfg.InstallVelero {
				if countIt == 0 {
					Expect(VeleroInstall(context.Background(), &VeleroCfg, useVolumeSnapshots)).To(Succeed())
				}
				countIt++
			}
		})

		AfterEach(func() {
			if !VeleroCfg.Debug {
				if VeleroCfg.InstallVelero {
					if countIt == len(tests) && !VeleroCfg.Debug {
						Expect(VeleroUninstall(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)).To((Succeed()))
					}
				}
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
	return nil
}

func (t *TestCase) CreateResources() error {
	return nil
}

func (t *TestCase) StartRun() error {
	return nil
}

func (t *TestCase) Backup() error {
	if err := VeleroBackupExec(t.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, t.BackupName, t.BackupArgs); err != nil {
		RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, t.BackupName, "")
		return errors.Wrapf(err, "Failed to backup resources")
	}
	return nil
}

func (t *TestCase) Destroy() error {
	err := CleanupNamespacesWithPoll(t.Ctx, t.Client, t.NSBaseName)
	if err != nil {
		return errors.Wrap(err, "Could cleanup retrieve namespaces")
	}
	return nil
}

func (t *TestCase) Restore() error {
	// the snapshots of AWS may be still in pending status when do the restore, wait for a while
	// to avoid this https://github.com/vmware-tanzu/velero/issues/1799
	// TODO remove this after https://github.com/vmware-tanzu/velero/issues/3533 is fixed
	if t.UseVolumeSnapshots {
		fmt.Println("Waiting 5 minutes to make sure the snapshots are ready...")
		time.Sleep(5 * time.Minute)
	}

	By("Start to restore ......", func() {
		quitCh := make(chan struct{})
		if strings.Contains(t.RestoreName, "-opt-") {
			for _, ns := range *t.NSIncluded {
				go func() {
					for {
						select {
						case <-quitCh:
							return
						default:
						}
						fmt.Printf("start to get log for namespace %s ......", ns)
						arg0 := []string{"-u"}
						KubectlGetInfo("date", arg0)
						arg := []string{"get", "all", "-n", ns}
						KubectlGetInfo("kubectl", arg)
						time.Sleep(5 * time.Second)
						arg1 := []string{"get", "pvc", "-n", ns}
						KubectlGetInfo("kubectl", arg1)
						time.Sleep(5 * time.Second)
						arg2 := []string{"get", "pv"}
						KubectlGetInfo("kubectl", arg2)
						time.Sleep(5 * time.Second)
						arg3 := []string{"get", "events", "-o", "custom-columns=FirstSeen:.firstTimestamp,Count:.count,From:.source.component,Type:.type,Reason:.reason,Message:.message", "--all-namespaces"}
						KubectlGetInfo("kubectl", arg3)
						time.Sleep(20 * time.Second)
					}
				}()
			}
		}
		var err error
		if err = VeleroRestoreExec(t.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, t.RestoreName, t.RestoreArgs); err != nil {
			RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", t.RestoreName)
		}
		close(quitCh)
		Expect(err).To(BeNil())
		// Expect(VeleroRestoreExec(t.Ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, t.RestoreName, t.RestoreArgs)).To(Succeed(), func() string {
		// 	RunDebug(context.Background(), VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, "", t.RestoreName)
		// 	return "Fail to restore workload"
		// })
	})
	return nil
}

func (t *TestCase) Verify() error {
	return nil
}

func (t *TestCase) Clean() error {
	if !VeleroCfg.Debug {
		By(fmt.Sprintf("Clean namespace with prefix %s after test", t.NSBaseName), func() {
			CleanupNamespaces(t.Ctx, t.Client, t.NSBaseName)
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
	fmt.Printf("Running test case %s\n", test.GetTestMsg().Desc)
	if test == nil {
		return errors.New("No case should be tested")
	}

	defer test.Clean()
	err := test.StartRun()
	if err != nil {
		return err
	}
	err = test.CreateResources()
	if err != nil {
		return err
	}
	err = test.Backup()
	if err != nil {
		return err
	}
	err = test.Destroy()
	if err != nil {
		return err
	}
	err = test.Restore()
	if err != nil {
		return err
	}
	err = test.Verify()
	if err != nil {
		return err
	}
	return nil
}
