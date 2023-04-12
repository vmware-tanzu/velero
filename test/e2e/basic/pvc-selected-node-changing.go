package basic

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type PVCSelectedNodeChanging struct {
	TestCase
	labels         map[string]string
	data           map[string]string
	configmaptName string
	namespace      string
	oldNodeName    string
	newNodeName    string
	volume         string
	podName        string
	mappedNS       string
	pvcName        string
	ann            string
}

const PSNCBaseName string = "psnc-"

var PVCSelectedNodeChangingTest func() = TestFunc(&PVCSelectedNodeChanging{
	namespace: PSNCBaseName + "1", TestCase: TestCase{NSBaseName: PSNCBaseName}})

func (p *PVCSelectedNodeChanging) Init() error {
	p.VeleroCfg = VeleroCfg
	p.Client = *p.VeleroCfg.ClientToInstallVelero
	p.NSBaseName = PSNCBaseName
	p.namespace = p.NSBaseName + UUIDgen.String()
	p.mappedNS = p.namespace + "-mapped"
	p.TestMsg = &TestMSG{
		Desc:      "Changing PVC node selector",
		FailedMSG: "Failed to changing PVC node selector",
		Text:      "Change node selectors of persistent volume claims during restores",
	}
	p.BackupName = "backup-sc-" + UUIDgen.String()
	p.RestoreName = "restore-" + UUIDgen.String()
	p.labels = map[string]string{"velero.io/plugin-config": "",
		"velero.io/change-pvc-node-selector": "RestoreItemAction"}
	p.configmaptName = "change-pvc-node-selector-config"
	p.volume = "volume-1"
	p.podName = "pod-1"
	p.pvcName = "pvc-1"
	p.ann = "volume.kubernetes.io/selected-node"
	return nil
}

func (p *PVCSelectedNodeChanging) StartRun() error {
	p.BackupName = p.BackupName + "backup-" + UUIDgen.String()
	p.RestoreName = p.RestoreName + "restore-" + UUIDgen.String()
	p.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", p.BackupName,
		"--include-namespaces", p.namespace,
		"--snapshot-volumes=false", "--wait",
	}
	p.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", p.RestoreName,
		"--from-backup", p.BackupName, "--namespace-mappings", fmt.Sprintf("%s:%s", p.namespace, p.mappedNS), "--wait",
	}
	return nil
}
func (p *PVCSelectedNodeChanging) CreateResources() error {
	p.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)
	By(fmt.Sprintf("Create namespace %s", p.namespace), func() {
		Expect(CreateNamespace(context.Background(), p.Client, p.namespace)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s", p.namespace))
	})

	By(fmt.Sprintf("Create pod %s in namespace %s", p.podName, p.namespace), func() {
		nodeNameList, err := GetWorkerNodes(context.Background())
		Expect(err).To(Succeed())
		for _, nodeName := range nodeNameList {
			p.oldNodeName = nodeName
			fmt.Printf("Create PVC on node %s\n", p.oldNodeName)
			pvcAnn := map[string]string{p.ann: nodeName}
			_, err := CreatePod(p.Client, p.namespace, p.podName, "default", p.pvcName, []string{p.volume}, pvcAnn, nil)
			Expect(err).To(Succeed())
			err = WaitForPods(context.Background(), p.Client, p.namespace, []string{p.podName})
			Expect(err).To(Succeed())
			break
		}
	})

	By("Prepare ConfigMap data", func() {
		nodeNameList, err := GetWorkerNodes(context.Background())
		Expect(err).To(Succeed())
		Expect(len(nodeNameList) > 2).To(Equal(true))
		for _, nodeName := range nodeNameList {
			if nodeName != p.oldNodeName {
				p.newNodeName = nodeName
				break
			}
		}
		p.data = map[string]string{p.oldNodeName: p.newNodeName}
	})

	By(fmt.Sprintf("Create ConfigMap %s in namespace %s", p.configmaptName, p.VeleroCfg.VeleroNamespace), func() {
		cm, err := CreateConfigMap(p.Client.ClientGo, p.VeleroCfg.VeleroNamespace, p.configmaptName, p.labels, p.data)
		Expect(err).To(Succeed(), fmt.Sprintf("failed to create configmap in the namespace %q", p.VeleroCfg.VeleroNamespace))
		fmt.Printf("Configmap: %v", cm)
	})
	return nil
}

func (p *PVCSelectedNodeChanging) Destroy() error {
	By(fmt.Sprintf("Start to destroy namespace %s......", p.NSBaseName), func() {
		Expect(CleanupNamespacesWithPoll(context.Background(), p.Client, p.NSBaseName)).To(Succeed(),
			fmt.Sprintf("Failed to delete namespace %s", p.NSBaseName))
	})
	return nil
}

func (p *PVCSelectedNodeChanging) Restore() error {
	By(fmt.Sprintf("Start to restore %s .....", p.RestoreName), func() {
		Expect(VeleroRestoreExec(context.Background(), p.VeleroCfg.VeleroCLI,
			p.VeleroCfg.VeleroNamespace, p.RestoreName,
			p.RestoreArgs, velerov1api.RestorePhaseCompleted)).To(
			Succeed(),
			func() string {
				RunDebug(context.Background(), p.VeleroCfg.VeleroCLI,
					p.VeleroCfg.VeleroNamespace, "", p.RestoreName)
				return "Fail to restore workload"
			})
		err := WaitForPods(p.Ctx, p.Client, p.mappedNS, []string{p.podName})
		Expect(err).To(Succeed())
	})
	return nil
}
func (p *PVCSelectedNodeChanging) Verify() error {
	By(fmt.Sprintf("PVC selected node should be %s", p.newNodeName), func() {
		pvcNameList, err := GetPvcByPodName(context.Background(), p.mappedNS, p.pvcName)
		Expect(err).To(Succeed())
		Expect(len(pvcNameList)).Should(Equal(1))
		pvc, err := GetPVC(context.Background(), p.Client, p.mappedNS, pvcNameList[0])
		Expect(err).To(Succeed())
		Expect(pvc.Annotations[p.ann]).To(Equal(p.newNodeName))
	})
	return nil
}
