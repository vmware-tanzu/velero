/*
Copyright 2021 the Velero contributors.

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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/providers"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

const fileName = "test-data.txt"

type BasicCase struct {
	TestCase
	PVCount int
}

var BasicCaseTest func() = TestFunc(&BasicCase{})

func (b *BasicCase) Init() error {
	b.TestCase.Init()
	b.VeleroCfg = VeleroCfg
	b.Client = *b.VeleroCfg.ClientToInstallVelero
	b.NamespacesTotal = 1
	b.PVCount = 1
	b.CaseBaseName = "basic-test-" + b.UUIDgen
	b.NSIncluded = &[]string{}
	for nsNum := 0; nsNum < b.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", b.CaseBaseName, nsNum)
		*b.NSIncluded = append(*b.NSIncluded, createNSName)
	}

	b.BackupName = "basic-snapshot-case-" + b.UUIDgen
	b.RestoreName = "basic-snapshot-case-" + b.UUIDgen

	b.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", b.BackupName,
		"--include-namespaces", strings.Join(*b.NSIncluded, ","),
		"--wait",
	}

	if b.VeleroCfg.UseVolumeSnapshots {
		b.BackupArgs = append(b.BackupArgs, "--default-volumes-to-fs-backup=false")
		b.BackupArgs = append(b.BackupArgs, "--snapshot-volumes")
	} else {
		b.BackupArgs = append(b.BackupArgs, "--default-volumes-to-fs-backup=true")
		b.BackupArgs = append(b.BackupArgs, "--snapshot-volumes=false")
	}

	b.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore", b.RestoreName,
		"--from-backup", b.BackupName, "--wait",
	}

	b.TestMsg = &TestMSG{
		Desc:      "Basic backup and restore test case",
		FailedMSG: "Failed to backup and restore of volume",
		Text:      fmt.Sprintf("Should backup and restore PVs in namespace %s", *b.NSIncluded),
	}
	return nil
}

func (b *BasicCase) CreateResources() error {
	b.Ctx, b.CtxCancel = context.WithTimeout(context.Background(), 20*time.Minute)
	By(("Installing storage class..."), func() {
		yamlFile := fmt.Sprintf("testdata/storage-class/%s.yaml", VeleroCfg.CloudProvider)
		if strings.EqualFold(b.VeleroCfg.CloudProvider, "azure") && strings.EqualFold(b.VeleroCfg.Features, "EnableCSI") {
			yamlFile = fmt.Sprintf("testdata/storage-class/%s-csi.yaml", VeleroCfg.CloudProvider)
		}
		Expect(InstallStorageClass(b.Ctx, yamlFile)).To(Succeed(), "Failed to install storage class")
	})

	for nsNum := 0; nsNum < b.NamespacesTotal; nsNum++ {
		namespace := fmt.Sprintf("%s-%00000d", b.CaseBaseName, nsNum)
		By(fmt.Sprintf("Create namespaces %s for workload\n", namespace), func() {
			Expect(CreateNamespace(b.Ctx, b.Client, namespace)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", namespace))
		})

		volName := fmt.Sprintf("vol-%s-%00000d", b.CaseBaseName, nsNum)
		volList := PrepareVolumeList([]string{volName})

		// Create PVC
		By(fmt.Sprintf("Creating pvc in namespaces ...%s\n", namespace), func() {
			Expect(b.createPVC(nsNum, namespace, volList)).To(Succeed(), fmt.Sprintf("Failed to create pvc in namespace %s", namespace))
		})

		// Create deployment
		By(fmt.Sprintf("Creating deployment in namespaces ...%s\n", namespace), func() {
			Expect(b.createDeploymentWithVolume(namespace, volList)).To(Succeed(), fmt.Sprintf("Failed to create deployment namespace %s", namespace))
		})

		//Write data into pods
		By(fmt.Sprintf("Writing data into pod in namespaces ...%s\n", namespace), func() {
			Expect(b.writeDataIntoPods(namespace, volName)).To(Succeed(), fmt.Sprintf("Failed to write data into pod in namespace %s", namespace))
		})
	}

	return nil
}

func (b *BasicCase) WaitForBackup() error {
	if !b.UseVolumeSnapshots {
		for index := 0; index < b.NamespacesTotal; index++ {
			ns := fmt.Sprintf("%s-%00000d", b.CaseBaseName, index)
			pvbs, err := GetPVB(b.Ctx, b.VeleroCfg.VeleroNamespace, ns)
			if err != nil {
				return errors.Wrapf(err, "failed to get PVB for namespace %s", ns)
			} else if len(pvbs) != b.PVCount {
				return errors.New(fmt.Sprintf("PVB count %d should be %d in namespace %s", len(pvbs), b.PVCount, ns))
			}
			if b.VeleroCfg.CloudProvider != "vsphere" {
				// wait for a period to confirm no snapshots exist for the backup
				if index == 0 {
					time.Sleep(5 * time.Minute)
				}
				snapshotCheckPoint, err := GetSnapshotCheckPoint(b.Client, b.VeleroCfg, 0,
					ns, b.BackupName, []string{"pvc-0"})
				if err != nil {
					return errors.Wrap(err, "failed to get snapshot checkPoint")
				}
				if !strings.EqualFold(b.VeleroCfg.Features, "EnableCSI") {
					err = SnapshotsShouldNotExistInCloud(b.VeleroCfg.CloudProvider,
						b.VeleroCfg.CloudCredentialsFile, b.VeleroCfg.BSLBucket, b.VeleroCfg.BSLConfig,
						b.BackupName, snapshotCheckPoint)
					if err != nil {
						return errors.Wrap(err, "exceed waiting for snapshot created in cloud")
					}
				}
			}
		}
		return nil
	}

	for index := 0; index < b.NamespacesTotal; index++ {
		ns := fmt.Sprintf("%s-%00000d", b.CaseBaseName, index)
		if b.VeleroCfg.CloudProvider == "vsphere" {
			// Wait for uploads started by the Velero Plug-in for vSphere to complete
			// TODO - remove after upload progress monitoring is implemented
			if err := WaitForVSphereUploadCompletion(b.Ctx, time.Hour, ns, 1); err != nil {
				return errors.Wrapf(err, "Error waiting for uploads to complete")
			}
		}

		point, err := GetSnapshotCheckPoint(b.Client, b.VeleroCfg, 1, ns, b.BackupName, []string{"pvc-0"})
		if err != nil {
			return errors.Wrap(err, "Fail to get snapshot checkpoint")
		}

		if err := SnapshotsShouldBeCreatedInCloud(b.VeleroCfg.CloudProvider,
			b.VeleroCfg.CloudCredentialsFile, b.VeleroCfg.BSLBucket, b.VeleroCfg.BSLConfig,
			b.BackupName, point); err != nil {
			return errors.Wrap(err, "exceed waiting for snapshot created in cloud")
		}
	}

	return nil
}

func (b *BasicCase) Verify() error {
	for i, ns := range *b.NSIncluded {
		By(fmt.Sprintf("Verify pod data in namespace %s", ns), func() {
			By(fmt.Sprintf("wait for ready deployment in namespace %s", ns), func() {
				err := WaitForReadyDeployment(b.Client.ClientGo, ns, b.CaseBaseName)
				Expect(err).To(Succeed(), fmt.Sprintf("failed to wait for ready deployment in namespace: %q with error %v", ns, err))
			})

			podList, err := ListPods(b.Ctx, b.Client, ns)
			Expect(err).To(Succeed(), fmt.Sprintf("failed to list pod in namespace: %q with error %v", ns, err))
			err = b.VerifyDataByNamespace(ns, ns, fmt.Sprintf("vol-%s-%00000d", b.CaseBaseName, i), podList)
			Expect(err).To(Succeed(), fmt.Sprintf("failed to verify pod volume data in namespace: %q with error %v", ns, err))
		})

	}
	return nil
}

func (b *BasicCase) VerifyDataByNamespace(ns, originalNS, volName string, podList *v1.PodList) error {
	for _, pod := range podList.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.Name != volName {
				continue
			}
			content, err := ReadFileFromPodVolume(b.Ctx, ns, pod.Name, "container-busybox", vol.Name, fileName)
			if err != nil {
				return fmt.Errorf("fail to read file %s from volume %s of pod %s in namespace %s",
					fileName, vol.Name, pod.Name, ns)
			}
			content = strings.Replace(content, "\n", "", -1)
			originContent := strings.Replace(fmt.Sprintf("ns-%s pod-%s volume-%s", originalNS, pod.Name, vol.Name), "\n", "", -1)
			if content != originContent {
				return fmt.Errorf("content of file %s does not equal to original in volume %s of pod %s in namespace %s",
					fileName, vol.Name, pod.Name, ns)
			}
		}
	}
	return nil
}

func (b *BasicCase) Clean() error {
	if err := DeleteStorageClass(b.Ctx, b.Client, "e2e-storage-class"); err != nil {
		return err
	}

	return b.GetTestCase().Clean()
}

func (b *BasicCase) createPVC(index int, namespace string, volList []*v1.Volume) error {
	var err error
	for i := range volList {
		pvcName := fmt.Sprintf("pvc-%d", i)
		By(fmt.Sprintf("Creating PVC %s in namespaces ...%s\n", pvcName, namespace))
		pvcBuilder := NewPVC(namespace, pvcName).WithStorageClass("e2e-storage-class").WithResourceStorage(resource.MustParse("1Mi"))
		err = CreatePvc(b.Client, pvcBuilder)
		if err != nil {
			return errors.Wrapf(err, "failed to create pvc %s in namespace %s", pvcName, namespace)
		}
	}
	return nil
}

func (b *BasicCase) createDeploymentWithVolume(namespace string, volList []*v1.Volume) error {
	deployment := NewDeployment(b.CaseBaseName, namespace, 1, map[string]string{"test-case": "basic"}, nil).WithVolume(volList).Result()
	deployment, err := CreateDeployment(b.Client.ClientGo, namespace, deployment)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create deloyment %s the namespace %q", deployment.Name, namespace))
	}
	err = WaitForReadyDeployment(b.Client.ClientGo, namespace, deployment.Name)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to wait for deployment %s to be ready in namespace: %q", deployment.Name, namespace))
	}
	return nil
}

func (b *BasicCase) writeDataIntoPods(namespace, volName string) error {
	podList, err := ListPods(b.Ctx, b.Client, namespace)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to list pods in namespace: %q with error %v", namespace, err))
	}
	for _, pod := range podList.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.Name != volName {
				continue
			}
			err := CreateFileToPod(b.Ctx, namespace, pod.Name, "container-busybox", vol.Name, fileName, fmt.Sprintf("ns-%s pod-%s volume-%s", namespace, pod.Name, vol.Name))
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to create file into pod %s in namespace: %q", pod.Name, namespace))
			}
		}
	}
	return nil
}
