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
	. "github.com/vmware-tanzu/velero/test/e2e/util/kibishii"
)

type NamespaceMapping struct {
	TestCase
	MappedNamespaceList []string
	kibishiiData        *KibishiiData
}

const NamespaceBaseName string = "ns-mp-"

var OneNamespaceMappingResticTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NSBaseName: NamespaceBaseName, NSIncluded: &[]string{NamespaceBaseName + "1"}, UseVolumeSnapshots: false}})
var MultiNamespacesMappingResticTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NSBaseName: NamespaceBaseName, NSIncluded: &[]string{NamespaceBaseName + "1", NamespaceBaseName + "2"}, UseVolumeSnapshots: false}})
var OneNamespaceMappingSnapshotTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NSBaseName: NamespaceBaseName, NSIncluded: &[]string{NamespaceBaseName + "1"}, UseVolumeSnapshots: true}})
var MultiNamespacesMappingSnapshotTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NSBaseName: NamespaceBaseName, NSIncluded: &[]string{NamespaceBaseName + "1", NamespaceBaseName + "2"}, UseVolumeSnapshots: true}})

func (n *NamespaceMapping) Init() error {
	//n.Client = TestClientInstance
	n.VeleroCfg = VeleroCfg
	n.Client = *n.VeleroCfg.ClientToInstallVelero
	n.VeleroCfg.UseVolumeSnapshots = n.UseVolumeSnapshots
	n.VeleroCfg.UseNodeAgent = !n.UseVolumeSnapshots
	n.kibishiiData = &KibishiiData{2, 10, 10, 1024, 1024, 0, 2}
	backupType := "restic"
	if n.UseVolumeSnapshots {
		backupType = "snapshot"
	}
	n.TestMsg = &TestMSG{
		Desc:      fmt.Sprintf("Restore namespace %s with namespace mapping by %s test", *n.NSIncluded, backupType),
		FailedMSG: "Failed to restore with namespace mapping",
		Text:      fmt.Sprintf("should restore namespace %s with namespace mapping by %s", *n.NSIncluded, backupType),
	}
	return nil
}

func (n *NamespaceMapping) StartRun() error {
	var mappedNS string
	var mappedNSList []string

	for index, ns := range *n.NSIncluded {
		mappedNS = mappedNS + ns + ":" + ns + UUIDgen.String()
		mappedNSList = append(mappedNSList, ns+UUIDgen.String())
		if index+1 != len(*n.NSIncluded) {
			mappedNS = mappedNS + ","
		}
		n.BackupName = n.BackupName + ns
		n.RestoreName = n.RestoreName + ns
	}
	n.BackupName = n.BackupName + UUIDgen.String()
	n.RestoreName = n.RestoreName + UUIDgen.String()

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
	return nil
}
func (n *NamespaceMapping) CreateResources() error {
	n.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)

	for index, ns := range *n.NSIncluded {
		n.kibishiiData.Levels = len(*n.NSIncluded) + index
		By(fmt.Sprintf("Creating namespaces ...%s\n", ns), func() {
			Expect(CreateNamespace(n.Ctx, n.Client, ns)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", ns))
		})
		By("Deploy sample workload of Kibishii", func() {
			Expect(KibishiiPrepareBeforeBackup(n.Ctx, n.Client, VeleroCfg.CloudProvider,
				ns, VeleroCfg.RegistryCredentialFile, VeleroCfg.Features,
				VeleroCfg.KibishiiDirectory, false, n.kibishiiData)).To(Succeed())
		})
	}
	return nil
}

func (n *NamespaceMapping) Verify() error {
	n.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	for index, ns := range n.MappedNamespaceList {
		n.kibishiiData.Levels = len(*n.NSIncluded) + index
		By(fmt.Sprintf("Verify workload %s after restore ", ns), func() {
			Expect(KibishiiVerifyAfterRestore(n.Client, ns,
				n.Ctx, n.kibishiiData)).To(Succeed(), "Fail to verify workload after restore")
		})
	}
	for _, ns := range *n.NSIncluded {
		By(fmt.Sprintf("Verify namespace %s for backup is no longer exist after restore with namespace mapping", ns), func() {
			Expect(NamespaceShouldNotExist(n.Ctx, n.Client, ns)).To(Succeed())
		})
	}
	return nil
}
