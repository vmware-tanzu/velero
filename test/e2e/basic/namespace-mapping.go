package basic

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

type NamespaceMapping struct {
	BasicSnapshotCase
	MappedNamespaceList []string
}

var NamespaceMappingResticTest func() = TestFuncWithMultiIt([]VeleroBackupRestoreTest{
	&NamespaceMapping{BasicSnapshotCase: BasicSnapshotCase{TestCase: TestCase{NamespacesTotal: 1, UseVolumeSnapshots: false}}},
	&NamespaceMapping{BasicSnapshotCase: BasicSnapshotCase{TestCase: TestCase{NamespacesTotal: 2, UseVolumeSnapshots: false}}}})

var NamespaceMappingSnapshotTest func() = TestFuncWithMultiIt([]VeleroBackupRestoreTest{
	&NamespaceMapping{BasicSnapshotCase: BasicSnapshotCase{TestCase: TestCase{NamespacesTotal: 1, UseVolumeSnapshots: true}}},
	&NamespaceMapping{BasicSnapshotCase: BasicSnapshotCase{TestCase: TestCase{NamespacesTotal: 2, UseVolumeSnapshots: true}}}})

func (n *NamespaceMapping) Init() error {
	n.TestCase.Init()
	n.VeleroCfg = VeleroCfg
	n.Client = *n.VeleroCfg.ClientToInstallVelero
	n.VeleroCfg.UseVolumeSnapshots = n.UseVolumeSnapshots
	n.VeleroCfg.UseNodeAgent = !n.UseVolumeSnapshots
	backupType := "restic"
	if n.UseVolumeSnapshots {
		backupType = "snapshot"
	}
	n.NSBaseName = "ns-mp-" + n.UUIDgen
	var mappedNS string
	var mappedNSList []string
	n.NSIncluded = &[]string{}
	for nsNum := 0; nsNum < n.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", n.NSBaseName, nsNum)
		*n.NSIncluded = append(*n.NSIncluded, createNSName)
		mappedNS = mappedNS + createNSName + ":" + createNSName + "-mapped"
		mappedNSList = append(mappedNSList, createNSName+"-mapped")
	}
	mappedNS = strings.TrimRightFunc(mappedNS, func(r rune) bool {
		return r == ','
	})

	n.BackupName = n.NSBaseName + "-backup-" + n.UUIDgen
	n.RestoreName = n.NSBaseName + "-restore-" + n.UUIDgen

	n.MappedNamespaceList = mappedNSList
	fmt.Println(mappedNSList)
	n.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", n.BackupName,
		"--include-namespaces", strings.Join(*n.NSIncluded, ","), "--wait",
	}
	if n.UseVolumeSnapshots {
		n.BackupArgs = append(n.BackupArgs, "--snapshot-volumes")
	} else {
		n.BackupArgs = append(n.BackupArgs, "--snapshot-volumes=false")
		n.BackupArgs = append(n.BackupArgs, "--default-volumes-to-fs-backup")
	}
	n.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", n.RestoreName,
		"--from-backup", n.BackupName, "--namespace-mappings", mappedNS,
		"--wait",
	}
	var ctxCancel context.CancelFunc
	n.Ctx, ctxCancel = context.WithTimeout(context.Background(), 60*time.Minute)
	defer ctxCancel()

	n.TestMsg = &TestMSG{
		Desc:      fmt.Sprintf("Restore namespace %s with namespace mapping by %s test", *n.NSIncluded, backupType),
		FailedMSG: "Failed to restore with namespace mapping",
		Text:      fmt.Sprintf("should restore namespace %s with namespace mapping by %s", *n.NSIncluded, backupType),
	}
	return nil
}

func (n *NamespaceMapping) verifyDataByNamespace(ns, mappedNS, volName string) error {
	err := WaitForReadyDeployment(n.Client.ClientGo, mappedNS, n.NSBaseName)
	if err != nil {
		return fmt.Errorf("Waiting for deployment %s in namespace %s ready", n.NSBaseName, ns)
	}
	podList, err := ListPods(n.Ctx, n.Client, mappedNS)
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace: %q with error %v", ns, err)
	}
	return n.VerifyDataByNamespace(mappedNS, ns, volName, podList)
}

func (n *NamespaceMapping) Verify() error {
	if len(*n.NSIncluded) != len(n.MappedNamespaceList) {
		return fmt.Errorf("the namespace has different mapping namespace length")
	}
	for index, ns := range n.MappedNamespaceList {
		err := n.verifyDataByNamespace((*n.NSIncluded)[index], ns, fmt.Sprintf("vol-%s-%00000d", n.NSBaseName, index))
		Expect(err).To(Succeed(), fmt.Sprintf("failed to verify pod volume data in namespace: %q with error %v", ns, err))
	}
	for _, ns := range *n.NSIncluded {
		By(fmt.Sprintf("Verify namespace %s for backup is no longer exist after restore with namespace mapping", ns), func() {
			Expect(NamespaceShouldNotExist(n.Ctx, n.Client, ns)).To(Succeed())
		})
	}
	return nil
}

func (n *NamespaceMapping) Clean() error {
	if err := DeleteStorageClass(n.Ctx, n.Client, "e2e-storage-class"); err != nil {
		return err
	}
	for _, ns := range n.MappedNamespaceList {
		if err := DeleteNamespace(n.Ctx, n.Client, ns, false); err != nil {
			return err
		}
	}

	return n.GetTestCase().Clean()
}
