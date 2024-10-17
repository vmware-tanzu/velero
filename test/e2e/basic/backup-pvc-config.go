package basic

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

type BackupPVCConfigChange struct {
	TestCase
	data                      map[string]string
	configmapName             string
	namespace                 string
	sourcePVCStorageClassName string
	backupPVCStorageClassName string
	pvcName                   string
	volumeName                string
	podName                   string
	deploymentName            string
}

const BPCCBaseName string = "bpcc"

var BackupPVCConfigChangeTest = TestFunc(&BackupPVCConfigChange{})
var configData = map[string]string{
	"backupPVC": `
        {
            "e2e-storage-class": {
                "storageClass": "e2e-storage-class-2",
                "readOnly": true
            },
        }`,
}

func (b *BackupPVCConfigChange) Init() error {
	b.TestCase.Init()
	b.configmapName = "node-agent-configmap"
	b.VeleroCfg.Options.NodeAgentConfigMap = b.configmapName
	b.CaseBaseName = BPCCBaseName + b.UUIDgen
	b.namespace = b.CaseBaseName
	b.BackupName = "backup-" + b.CaseBaseName
	b.RestoreName = "restore-" + b.CaseBaseName
	b.sourcePVCStorageClassName = StorageClassName
	b.backupPVCStorageClassName = StorageClassName2
	b.data = configData
	b.volumeName = "volume-1"
	b.pvcName = fmt.Sprintf("pvc-%s", b.volumeName)
	b.podName = "pod-1"
	b.BackupArgs = []string{
		"create", "--namespace", b.VeleroCfg.VeleroNamespace, "backup", b.BackupName,
		"--include-namespaces", b.namespace,
		"--snapshot-move-data=true", "--wait",
	}
	b.TestMsg = &TestMSG{
		Desc:      "BackupPVC Config Change",
		FailedMSG: "Failed to apply Backup PVC Config",
		Text:      "Change the configuration of backupPVC during DM backup operation",
	}
	return nil
}

func (s *BackupPVCConfigChange) CreateResources() error {
	label := map[string]string{
		"app": "test",
	}

	By(("Installing storage class..."), func() {
		Expect(InstallTestStorageClasses(fmt.Sprintf("../testdata/storage-class/%s.yaml", s.VeleroCfg.CloudProvider))).To(Succeed(), "Failed to install storage class")
	})

	By(fmt.Sprintf("Create namespace %s", s.namespace), func() {
		Expect(CreateNamespace(s.Ctx, s.Client, s.namespace)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s", s.namespace))
	})

	By(fmt.Sprintf("Create a deployment in namespace %s", s.VeleroCfg.VeleroNamespace), func() {
		pvc, err := CreatePVC(s.Client, s.namespace, s.pvcName, s.sourcePVCStorageClassName, nil)
		Expect(err).To(Succeed())
		vols := CreateVolumes(pvc.Name, []string{s.volumeName})

		deployment := NewDeployment(s.CaseBaseName, s.namespace, 1, label, nil).WithVolume(vols).Result()
		deployment, err = CreateDeployment(s.Client.ClientGo, s.namespace, deployment)
		Expect(err).To(Succeed())
		s.deploymentName = deployment.Name
		err = WaitForReadyDeployment(s.Client.ClientGo, s.namespace, s.deploymentName)
		Expect(err).To(Succeed())
	})

	By(fmt.Sprintf("Create ConfigMap %s in namespace %s", s.configmapName, s.VeleroCfg.VeleroNamespace), func() {
		_, err := CreateConfigMap(s.Client.ClientGo, s.VeleroCfg.VeleroNamespace, s.configmapName, label, s.data)
		Expect(err).To(Succeed(), fmt.Sprintf("failed to create configmap in the namespace %q", s.VeleroCfg.VeleroNamespace))
	})
	return nil
}

func (b *BackupPVCConfigChange) Verify() error {
	return nil
}

func (b *BackupPVCConfigChange) Cleanup() error {
	return nil
}

func (b *BackupPVCConfigChange) Restore() error {
	return nil
}

func (b *BackupPVCConfigChange) Destroy() error {
	return nil
}
