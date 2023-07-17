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

type StorageClasssChanging struct {
	TestCase
	labels          map[string]string
	data            map[string]string
	configmaptName  string
	namespace       string
	srcStorageClass string
	desStorageClass string
	volume          string
	podName         string
	mappedNS        string
}

const SCCBaseName string = "scc-"

var StorageClasssChangingTest func() = TestFunc(&StorageClasssChanging{})

func (s *StorageClasssChanging) Init() error {
	s.TestCase.Init()
	s.CaseBaseName = SCCBaseName + s.UUIDgen
	s.namespace = s.CaseBaseName
	s.BackupName = "backup-" + s.CaseBaseName
	s.RestoreName = "restore-" + s.CaseBaseName
	s.mappedNS = s.namespace + "-mapped"
	s.VeleroCfg = VeleroCfg
	s.Client = *s.VeleroCfg.ClientToInstallVelero
	s.TestMsg = &TestMSG{
		Desc:      "Changing PV/PVC Storage Classes",
		FailedMSG: "Failed to changing PV/PVC Storage Classes",
		Text: "Change the storage class of persistent volumes and persistent" +
			" volume claims during restores",
	}
	s.srcStorageClass = "default"
	s.desStorageClass = StorageClassName
	s.labels = map[string]string{"velero.io/change-storage-class": "RestoreItemAction",
		"velero.io/plugin-config": ""}
	s.data = map[string]string{s.srcStorageClass: s.desStorageClass}
	s.configmaptName = "change-storage-class-config"
	s.volume = "volume-1"
	s.podName = "pod-1"
	s.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", s.BackupName,
		"--include-namespaces", s.namespace,
		"--snapshot-volumes=false", "--wait",
	}
	s.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", s.RestoreName,
		"--from-backup", s.BackupName, "--namespace-mappings", fmt.Sprintf("%s:%s", s.namespace, s.mappedNS), "--wait",
	}
	return nil
}
func (s *StorageClasssChanging) CreateResources() error {
	s.Ctx, s.CtxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
	By(fmt.Sprintf("Create a storage class %s", s.desStorageClass), func() {
		Expect(InstallStorageClass(s.Ctx, fmt.Sprintf("testdata/storage-class/%s.yaml",
			s.VeleroCfg.CloudProvider))).To(Succeed())
	})
	By(fmt.Sprintf("Create namespace %s", s.namespace), func() {
		Expect(CreateNamespace(s.Ctx, s.Client, s.namespace)).To(Succeed(),
			fmt.Sprintf("Failed to create namespace %s", s.namespace))
	})

	By(fmt.Sprintf("Create pod %s in namespace %s", s.podName, s.namespace), func() {
		_, err := CreatePod(s.Client, s.namespace, s.podName, s.srcStorageClass, "", []string{s.volume}, nil, nil)
		Expect(err).To(Succeed())
	})
	By(fmt.Sprintf("Create ConfigMap %s in namespace %s", s.configmaptName, s.VeleroCfg.VeleroNamespace), func() {
		_, err := CreateConfigMap(s.Client.ClientGo, s.VeleroCfg.VeleroNamespace, s.configmaptName, s.labels, s.data)
		Expect(err).To(Succeed(), fmt.Sprintf("failed to create configmap in the namespace %q", s.VeleroCfg.VeleroNamespace))
	})
	return nil
}

func (s *StorageClasssChanging) Destroy() error {
	By(fmt.Sprintf("Expect storage class of PV %s to be %s ", s.volume, s.srcStorageClass), func() {
		pvName, err := GetPVByPodName(s.Client, s.namespace, s.volume)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to get PV name by pod name %s", s.podName))
		pv, err := GetPersistentVolume(s.Ctx, s.Client, s.namespace, pvName)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to get PV by pod name %s", s.podName))
		fmt.Println(pv)
		Expect(pv.Spec.StorageClassName).To(Equal(s.srcStorageClass),
			fmt.Sprintf("PV storage %s is not as expected %s", pv.Spec.StorageClassName, s.srcStorageClass))
	})

	By(fmt.Sprintf("Start to destroy namespace %s......", s.CaseBaseName), func() {
		Expect(CleanupNamespacesWithPoll(s.Ctx, s.Client, s.CaseBaseName)).To(Succeed(),
			fmt.Sprintf("Failed to delete namespace %s", s.CaseBaseName))
	})
	return nil
}

func (s *StorageClasssChanging) Restore() error {
	By(fmt.Sprintf("Start to restore %s .....", s.RestoreName), func() {
		Expect(VeleroRestoreExec(s.Ctx, s.VeleroCfg.VeleroCLI,
			s.VeleroCfg.VeleroNamespace, s.RestoreName,
			s.RestoreArgs, velerov1api.RestorePhaseCompleted)).To(
			Succeed(),
			func() string {
				RunDebug(context.Background(), s.VeleroCfg.VeleroCLI,
					s.VeleroCfg.VeleroNamespace, "", s.RestoreName)
				return "Fail to restore workload"
			})
	})
	return nil
}
func (s *StorageClasssChanging) Verify() error {
	By(fmt.Sprintf("Expect storage class of PV %s to be %s ", s.volume, s.desStorageClass), func() {
		Expect(WaitForPods(s.Ctx, s.Client, s.mappedNS, []string{s.podName})).To(Succeed(), fmt.Sprintf("Failed to wait pod ready %s", s.podName))
		pvName, err := GetPVByPodName(s.Client, s.mappedNS, s.volume)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to get PV name by pod name %s", s.podName))
		pv, err := GetPersistentVolume(s.Ctx, s.Client, s.mappedNS, pvName)
		Expect(err).To(Succeed(), fmt.Sprintf("Failed to get PV by pod name %s", s.podName))
		fmt.Println(pv)
		Expect(pv.Spec.StorageClassName).To(Equal(s.desStorageClass),
			fmt.Sprintf("PV storage %s is not as expected %s", pv.Spec.StorageClassName, s.desStorageClass))
	})
	return nil
}

func (s *StorageClasssChanging) Clean() error {
	if !s.VeleroCfg.Debug {
		DeleteConfigmap(s.Client.ClientGo, s.VeleroCfg.VeleroNamespace, s.configmaptName)
		DeleteStorageClass(s.Ctx, s.Client, s.desStorageClass)
		s.TestCase.Clean()
	}
	return nil
}
