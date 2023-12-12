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

package basic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

type BackupVolumeInfo struct {
	TestCase
	SnapshotVolumes          bool
	DefaultVolumesToFSBackup bool
	SnapshotMoveData         bool
	TimeoutDuration          time.Duration
}

func (v *BackupVolumeInfo) Init() error {
	v.TestCase.Init()

	BeforeEach(func() {
		if strings.Contains(v.VeleroCfg.Features, "EnableCSI") {
			if strings.Contains(v.CaseBaseName, "native-snapshot") {
				fmt.Printf("Skip native snapshot case %s when the CSI feature is enabled.\n", v.CaseBaseName)
				Skip("Skip due to vSphere CSI driver long time issue of Static provisioning")
			}
		} else {
			if strings.Contains(v.CaseBaseName, "csi") {
				fmt.Printf("Skip CSI related case %s when the CSI feature is not enabled.\n", v.CaseBaseName)
				Skip("Skip due to vSphere CSI driver long time issue of Static provisioning")
			}
		}
	})

	v.CaseBaseName = v.CaseBaseName + v.UUIDgen
	v.BackupName = "backup-" + v.CaseBaseName
	v.RestoreName = "restore-" + v.CaseBaseName
	v.TimeoutDuration = 10 * time.Minute
	v.NamespacesTotal = 1

	v.VeleroCfg = VeleroCfg
	v.Client = *v.VeleroCfg.ClientToInstallVelero
	v.NSIncluded = &[]string{v.CaseBaseName}

	v.TestMsg = &TestMSG{
		Desc:      "Test backup's VolumeInfo metadata content",
		Text:      "The VolumeInfo should be generated based on the backup type",
		FailedMSG: "Failed to verify the backup VolumeInfo's content",
	}

	v.BackupArgs = []string{
		"backup", "create", v.BackupName,
		"--namespace", v.VeleroCfg.VeleroNamespace,
		"--include-namespaces", v.CaseBaseName,
		"--snapshot-volumes" + "=" + strconv.FormatBool(v.SnapshotVolumes),
		"--default-volumes-to-fs-backup" + "=" + strconv.FormatBool(v.DefaultVolumesToFSBackup),
		"--snapshot-move-data" + "=" + strconv.FormatBool(v.SnapshotMoveData),
		"--wait",
	}

	v.RestoreArgs = []string{
		"create", "--namespace", v.VeleroCfg.VeleroNamespace, "restore", v.RestoreName,
		"--from-backup", v.BackupName, "--wait",
	}
	return nil
}

func (v *BackupVolumeInfo) CreateResources() error {
	v.Ctx, v.CtxCancel = context.WithTimeout(context.Background(), v.TimeoutDuration)
	labels := map[string]string{
		"volume-info": "true",
	}
	for nsNum := 0; nsNum < v.NamespacesTotal; nsNum++ {
		fmt.Printf("Creating namespaces ...\n")
		createNSName := v.CaseBaseName
		if err := CreateNamespaceWithLabel(v.Ctx, v.Client, createNSName, labels); err != nil {
			return errors.Wrapf(err, "Failed to create namespace %s", createNSName)
		}

		Expect(InstallTestStorageClasses(fmt.Sprintf("../testdata/storage-class/%s-csi.yaml", v.VeleroCfg.CloudProvider))).To(Succeed(), "Failed to install StorageClass")

		// Install VolumeSnapshotClass
		Expect(KubectlApplyByFile(v.Ctx, fmt.Sprintf("../testdata/volume-snapshot-class/%s.yaml", v.VeleroCfg.CloudProvider))).To(Succeed(), "Failed to install VolumeSnapshotClass")

		pvc, err := CreatePVC(v.Client, createNSName, "volume-info", CSIStorageClassName, nil)
		Expect(err).To(Succeed())
		vols := CreateVolumes(pvc.Name, []string{"volume-info"})

		//Create deployment
		fmt.Printf("Creating deployment in namespaces ...%s\n", createNSName)
		deployment := NewDeployment(v.CaseBaseName, createNSName, 1, labels, nil).WithVolume(vols).Result()
		deployment, err = CreateDeployment(v.Client.ClientGo, createNSName, deployment)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to delete the namespace %q", createNSName))
		}
		err = WaitForReadyDeployment(v.Client.ClientGo, createNSName, deployment.Name)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to ensure job completion in namespace: %q", createNSName))
		}
	}
	return nil
}

func (v *BackupVolumeInfo) Destroy() error {
	err := CleanupNamespaces(v.Ctx, v.Client, v.CaseBaseName)
	if err != nil {
		return errors.Wrap(err, "Could cleanup retrieve namespaces")
	}

	return WaitAllSelectedNSDeleted(v.Ctx, v.Client, "ns-test=true")
}

func (v *BackupVolumeInfo) cleanResource() error {
	if err := DeleteStorageClass(v.Ctx, v.Client, CSIStorageClassName); err != nil {
		return errors.Wrap(err, "fail to delete the StorageClass")
	}

	if err := KubectlDeleteByFile(v.Ctx, fmt.Sprintf("../testdata/volume-snapshot-class/%s.yaml", v.VeleroCfg.CloudProvider)); err != nil {
		return errors.Wrap(err, "fail to delete the VolumeSnapshotClass")
	}
	return nil
}
