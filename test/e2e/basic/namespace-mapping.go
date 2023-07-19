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

var OneNamespaceMappingResticTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NamespacesTotal: 1, UseVolumeSnapshots: false}})
var MultiNamespacesMappingResticTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NamespacesTotal: 2, UseVolumeSnapshots: false}})
var OneNamespaceMappingSnapshotTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NamespacesTotal: 1, UseVolumeSnapshots: true}})
var MultiNamespacesMappingSnapshotTest func() = TestFunc(&NamespaceMapping{TestCase: TestCase{NamespacesTotal: 2, UseVolumeSnapshots: true}})

func (n *NamespaceMapping) Init() error {
	n.TestCase.Init()
	n.CaseBaseName = "ns-mp-" + n.UUIDgen
	n.BackupName = "backup-" + n.CaseBaseName
	n.RestoreName = "restore-" + n.CaseBaseName
	n.VeleroCfg = VeleroCfg
	n.Client = *n.VeleroCfg.ClientToInstallVelero
	n.VeleroCfg.UseVolumeSnapshots = n.UseVolumeSnapshots
	n.VeleroCfg.UseNodeAgent = !n.UseVolumeSnapshots
	n.kibishiiData = &KibishiiData{Levels: 2, DirsPerLevel: 10, FilesPerLevel: 10, FileLength: 1024, BlockSize: 1024, PassNum: 0, ExpectedNodes: 2}
	backupType := "restic"
	if n.UseVolumeSnapshots {
		backupType = "snapshot"
	}
	var mappedNS string
	var mappedNSList []string
	n.NSIncluded = &[]string{}
	for nsNum := 0; nsNum < n.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", n.CaseBaseName, nsNum)
		*n.NSIncluded = append(*n.NSIncluded, createNSName)
		mappedNS = mappedNS + createNSName + ":" + createNSName + "-mapped"
		mappedNSList = append(mappedNSList, createNSName+"-mapped")
		mappedNS = mappedNS + ","
	}
	mappedNS = strings.TrimRightFunc(mappedNS, func(r rune) bool {
		return r == ','
	})

	n.TestMsg = &TestMSG{
		Desc:      fmt.Sprintf("Restore namespace %s with namespace mapping by %s test", *n.NSIncluded, backupType),
		FailedMSG: "Failed to restore with namespace mapping",
		Text:      fmt.Sprintf("should restore namespace %s with namespace mapping by %s", *n.NSIncluded, backupType),
	}
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
	n.Ctx, n.CtxCancel = context.WithTimeout(context.Background(), 60*time.Minute)
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

func (n *NamespaceMapping) Clean() error {
	if !n.VeleroCfg.Debug {
		if err := DeleteStorageClass(context.Background(), n.Client, KibishiiStorageClassName); err != nil {
			return err
		}
		for _, ns := range n.MappedNamespaceList {
			if err := DeleteNamespace(context.Background(), n.Client, ns, false); err != nil {
				return err
			}
		}

		return n.GetTestCase().Clean()
	}
	return nil
}
