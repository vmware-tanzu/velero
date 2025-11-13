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

package resourcemodifiers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	"github.com/vmware-tanzu/velero/test/util/common"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
)

// PVC size modifier test configuration for different backup methods
type PVCSizeModifierCase struct {
	TestCase
	cmName                   string
	yamlConfig               string
	statefulSetName          string
	pvcName                  string
	originalSize             string
	modifiedSize             string
	useVolumeSnapshots       bool
	snapshotMoveData         bool
	defaultVolumesToFSBackup bool
}

// Test functions for different backup methods
var CSISnapshotPVCSizeModifierTest func() = TestFunc(&PVCSizeModifierCase{
	originalSize:             "1Gi",
	modifiedSize:             "5Gi",
	useVolumeSnapshots:       true,
	snapshotMoveData:         false,
	defaultVolumesToFSBackup: false,
})

var KopiaFSBackupPVCSizeModifierTest func() = TestFunc(&PVCSizeModifierCase{
	originalSize:             "1Gi",
	modifiedSize:             "5Gi",
	useVolumeSnapshots:       false,
	snapshotMoveData:         false,
	defaultVolumesToFSBackup: true,
})

var CSIDataMoverPVCSizeModifierTest func() = TestFunc(&PVCSizeModifierCase{
	originalSize:             "1Gi",
	modifiedSize:             "5Gi",
	useVolumeSnapshots:       true,
	snapshotMoveData:         true,
	defaultVolumesToFSBackup: false,
})

func (p *PVCSizeModifierCase) Init() error {
	// Generate random number as UUIDgen and set one default timeout duration
	p.TestCase.Init()

	// Generate variable names based on CaseBaseName + UUIDgen
	p.CaseBaseName = "pvc-size-modifier-" + p.UUIDgen
	p.BackupName = "backup-" + p.CaseBaseName
	p.RestoreName = "restore-" + p.CaseBaseName
	p.cmName = "cm-" + p.CaseBaseName
	p.statefulSetName = "statefulset-" + p.CaseBaseName
	p.pvcName = "data-" + p.statefulSetName + "-0" // StatefulSet PVC naming pattern

	// Generate namespaces
	p.NamespacesTotal = 1
	p.NSIncluded = &[]string{}
	for nsNum := 0; nsNum < p.NamespacesTotal; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", p.CaseBaseName, nsNum)
		*p.NSIncluded = append(*p.NSIncluded, createNSName)
	}

	// Generate resource modifier YAML for PVC size modification
	p.yamlConfig = fmt.Sprintf(`
version: v1
resourceModifierRules:
- conditions:
    groupResource: persistentvolumeclaims
    resourceNameRegex: "%s"
  patches:
  - operation: replace
    path: "/spec/resources/requests/storage"
    value: "%s"
`, p.pvcName, p.modifiedSize)

	// Configure backup method
	p.VeleroCfg.UseVolumeSnapshots = p.useVolumeSnapshots
	if p.snapshotMoveData {
		p.VeleroCfg.SnapshotMoveData = true
		p.VeleroCfg.Features = "EnableCSI"
	}
	if p.defaultVolumesToFSBackup {
		p.VeleroCfg.DefaultVolumesToFsBackup = true
		p.VeleroCfg.UseNodeAgent = true
	} else if p.useVolumeSnapshots {
		p.VeleroCfg.Features = "EnableCSI"
		p.VeleroCfg.UseNodeAgent = false
	}

	p.BackupArgs = []string{
		"create", "--namespace", p.VeleroCfg.VeleroNamespace, "backup", p.BackupName,
		"--include-namespaces", strings.Join(*p.NSIncluded, ","),
		"--wait",
	}

	if p.defaultVolumesToFSBackup {
		p.BackupArgs = append(p.BackupArgs, "--default-volumes-to-fs-backup")
	} else if p.snapshotMoveData {
		p.BackupArgs = append(p.BackupArgs, "--snapshot-move-data")
	} else if p.useVolumeSnapshots {
		p.BackupArgs = append(p.BackupArgs, "--snapshot-volumes")
	}

	p.RestoreArgs = []string{
		"create", "--namespace", p.VeleroCfg.VeleroNamespace, "restore", p.RestoreName,
		"--resource-modifier-configmap", p.cmName,
		"--from-backup", p.BackupName, "--wait",
	}

	// Message output by ginkgo
	backupMethod := "FileSystem Backup"
	if p.snapshotMoveData {
		backupMethod = "CSI DataMover"
	} else if p.useVolumeSnapshots {
		backupMethod = "CSI Snapshot"
	}

	p.TestMsg = &TestMSG{
		Desc:      fmt.Sprintf("Validate PVC size modification with resource modifiers using %s", backupMethod),
		FailedMSG: fmt.Sprintf("Failed to modify PVC size from %s to %s using resource modifiers with %s", p.originalSize, p.modifiedSize, backupMethod),
		Text:      fmt.Sprintf("Should successfully increase PVC size from %s to %s during restore with %s", p.originalSize, p.modifiedSize, backupMethod),
	}
	return nil
}

func (p *PVCSizeModifierCase) CreateResources() error {
	// Create resource modifier ConfigMap
	By(fmt.Sprintf("Create configmap %s in namespace %s for resource modifiers\n", p.cmName, p.VeleroCfg.VeleroNamespace), func() {
		Expect(CreateConfigMapFromYAMLData(p.Client.ClientGo, p.yamlConfig, p.cmName, p.VeleroCfg.VeleroNamespace)).To(Succeed(),
			fmt.Sprintf("Failed to create configmap %s in namespace %s\n", p.cmName, p.VeleroCfg.VeleroNamespace))
	})

	By(fmt.Sprintf("Waiting for configmap %s in namespace %s ready\n", p.cmName, p.VeleroCfg.VeleroNamespace), func() {
		Expect(WaitForConfigMapComplete(p.Client.ClientGo, p.VeleroCfg.VeleroNamespace, p.cmName)).To(Succeed(),
			fmt.Sprintf("Failed to wait configmap %s in namespace %s ready\n", p.cmName, p.VeleroCfg.VeleroNamespace))
	})

	// Create test namespace
	for nsNum := 0; nsNum < p.NamespacesTotal; nsNum++ {
		namespace := fmt.Sprintf("%s-%00000d", p.CaseBaseName, nsNum)
		nsLabels := make(map[string]string)
		if p.VeleroCfg.WorkerOS == common.WorkerOSWindows {
			nsLabels = map[string]string{
				"pod-security.kubernetes.io/enforce":         "privileged",
				"pod-security.kubernetes.io/enforce-version": "latest",
			}
		}

		By(fmt.Sprintf("Create namespace %s for workload\n", namespace), func() {
			Expect(CreateNamespaceWithLabel(p.Ctx, p.Client, namespace, nsLabels)).To(Succeed(),
				fmt.Sprintf("Failed to create namespace %s", namespace))
		})

		// Create StatefulSet with PVC
		By(fmt.Sprintf("Creating StatefulSet with PVC in namespace %s\n", namespace), func() {
			Expect(p.createStatefulSetWithPVC(namespace)).To(Succeed(),
				fmt.Sprintf("Failed to create StatefulSet in namespace %s", namespace))
		})

		// Write test data to the volume
		By(fmt.Sprintf("Writing test data to PVC in namespace %s\n", namespace), func() {
			Expect(p.writeTestDataToPVC(namespace)).To(Succeed(),
				fmt.Sprintf("Failed to write test data to PVC in namespace %s", namespace))
		})
	}
	return nil
}

func (p *PVCSizeModifierCase) Verify() error {
	for _, ns := range *p.NSIncluded {
		// Verify PVC size has been modified
		By(fmt.Sprintf("Verify PVC size has been modified to %s in namespace %s", p.modifiedSize, ns), func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel()

			// Get the PVC
			pvc, err := p.Client.ClientGo.CoreV1().PersistentVolumeClaims(ns).Get(ctx, p.pvcName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Failed to get PVC %s in namespace %s", p.pvcName, ns))

			// Check the requested storage size
			requestedSize := pvc.Spec.Resources.Requests[corev1api.ResourceStorage]
			expectedSize, err := resource.ParseQuantity(p.modifiedSize)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Failed to parse expected size %s", p.modifiedSize))

			Expect(requestedSize.Equal(expectedSize)).To(BeTrue(),
				fmt.Sprintf("PVC size mismatch: expected %s, got %s", p.modifiedSize, requestedSize.String()))

			// For CSI and DataMover, also verify the actual capacity if PVC is bound
			if p.useVolumeSnapshots || p.snapshotMoveData {
				if pvc.Status.Phase == corev1api.ClaimBound {
					actualCapacity := pvc.Status.Capacity[corev1api.ResourceStorage]
					// Actual capacity should be at least the requested size
					Expect(actualCapacity.Cmp(expectedSize)).To(BeNumerically(">=", 0), fmt.Sprintf("PVC actual capacity %s is less than requested %s", actualCapacity.String(), p.modifiedSize))
				}
			}
		})

		// Verify StatefulSet is running with the resized PVC
		By(fmt.Sprintf("Verify StatefulSet is running with resized PVC in namespace %s", ns), func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel()

			// Check StatefulSet is ready
			err := wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*2, false, func(ctx context.Context) (bool, error) {
				sts, err := p.Client.ClientGo.AppsV1().StatefulSets(ns).Get(ctx, p.statefulSetName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				return sts.Status.ReadyReplicas == *sts.Spec.Replicas, nil
			})
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("StatefulSet %s not ready in namespace %s", p.statefulSetName, ns))
		})

		// Verify test data integrity
		By(fmt.Sprintf("Verify test data integrity in namespace %s", ns), func() {
			Expect(p.verifyTestDataIntegrity(ns)).To(Succeed(),
				fmt.Sprintf("Test data verification failed in namespace %s", ns))
		})
	}
	return nil
}

func (p *PVCSizeModifierCase) Clean() error {
	// Clean up ConfigMap
	if CurrentSpecReport().Failed() && p.VeleroCfg.FailFast {
		fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
	} else {
		if err := DeleteConfigMap(p.Client.ClientGo, p.VeleroCfg.VeleroNamespace, p.cmName); err != nil {
			return err
		}
		return p.GetTestCase().Clean() // Clean up resources in test namespace
	}
	return nil
}

func (p *PVCSizeModifierCase) createStatefulSetWithPVC(namespace string) error {
	// Create a StatefulSet with volumeClaimTemplate
	statefulSet := &appsv1api.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.statefulSetName,
			Namespace: namespace,
		},
		Spec: appsv1api.StatefulSetSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": p.statefulSetName},
			},
			Template: corev1api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": p.statefulSetName},
				},
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable-alpine",
							VolumeMounts: []corev1api.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
							Command: []string{"/bin/sh"},
							Args:    []string{"-c", "while true; do sleep 30; done"},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1api.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1api.PersistentVolumeClaimSpec{
						AccessModes: []corev1api.PersistentVolumeAccessMode{
							corev1api.ReadWriteOnce,
						},
						Resources: corev1api.VolumeResourceRequirements{
							Requests: corev1api.ResourceList{
								corev1api.ResourceStorage: resource.MustParse(p.originalSize),
							},
						},
						StorageClassName: ptr.To(StorageClassName),
					},
				},
			},
		},
	}

	_, err := p.Client.ClientGo.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create StatefulSet %s in namespace %s", p.statefulSetName, namespace)
	}

	// Wait for StatefulSet to be ready
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	err = wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*3, false, func(ctx context.Context) (bool, error) {
		sts, err := p.Client.ClientGo.AppsV1().StatefulSets(namespace).Get(ctx, p.statefulSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return sts.Status.ReadyReplicas == *sts.Spec.Replicas, nil
	})

	if err != nil {
		return errors.Wrapf(err, "StatefulSet %s not ready in namespace %s", p.statefulSetName, namespace)
	}

	// Wait for PVC to be bound
	err = wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*2, false, func(ctx context.Context) (bool, error) {
		pvc, err := p.Client.ClientGo.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, p.pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1api.ClaimBound, nil
	})

	if err != nil {
		return errors.Wrapf(err, "PVC %s not bound in namespace %s", p.pvcName, namespace)
	}

	return nil
}

func (p *PVCSizeModifierCase) writeTestDataToPVC(namespace string) error {
	podName := p.statefulSetName + "-0" // StatefulSet pod naming pattern

	// Write a test file to the PVC using kubectl exec
	cmdArgs := []string{"exec", "-n", namespace, podName, "-c", "nginx", "--",
		"sh", "-c", "echo 'Test data for PVC size modification' > /data/testfile.txt && echo 'Size test successful' > /data/verify.txt"}

	cmd := exec.Command("kubectl", cmdArgs...)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to write test data to PVC: %s", stderr)
	}
	fmt.Printf("Write test data output: %s\n", stdout)

	return nil
}

func (p *PVCSizeModifierCase) verifyTestDataIntegrity(namespace string) error {
	podName := p.statefulSetName + "-0"

	// Verify test files exist and contain expected data using kubectl exec
	cmdArgs := []string{"exec", "-n", namespace, podName, "-c", "nginx", "--",
		"sh", "-c", "cat /data/testfile.txt && cat /data/verify.txt"}

	cmd := exec.Command("kubectl", cmdArgs...)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to verify test data: %s", stderr)
	}

	expectedContent := "Test data for PVC size modification"
	if !strings.Contains(stdout, expectedContent) {
		return errors.Errorf("test data mismatch: expected to find '%s', got: %s", expectedContent, stdout)
	}

	if !strings.Contains(stdout, "Size test successful") {
		return errors.Errorf("verification file missing or corrupted: %s", stdout)
	}

	return nil
}
