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
package bslmgmt

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
	. "github.com/vmware-tanzu/velero/test/util/kibishii"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

const (
	startupValidationTestNs = "bsl-startup-validation"
)

// BackupRepositoryStartupValidation tests that backup repositories are validated
// against BSL configuration on Velero startup and invalidated if configuration has changed
func BackupRepositoryStartupValidation() {
	var (
		veleroCfg VeleroConfig
	)
	veleroCfg = VeleroCfg
	veleroCfg.UseVolumeSnapshots = false
	veleroCfg.UseNodeAgent = true

	BeforeEach(func() {
		var err error
		flag.Parse()
		UUIDgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if InstallVelero {
			Expect(PrepareVelero(context.Background(), "BSL Startup Validation", veleroCfg)).To(Succeed())
		}
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() && veleroCfg.FailFast {
			fmt.Println("Test case failed and fail fast is enabled. Skip resource clean up.")
		} else {
			By("Clean backups after test", func() {
				veleroCfg.ClientToInstallVelero = veleroCfg.DefaultClient
				DeleteAllBackups(context.Background(), &veleroCfg)
			})
			By(fmt.Sprintf("Delete sample workload namespace %s", startupValidationTestNs), func() {
				Expect(DeleteNamespace(context.Background(), *veleroCfg.ClientToInstallVelero, startupValidationTestNs,
					true)).To(Succeed(), fmt.Sprintf("failed to delete the namespace %q",
					startupValidationTestNs))
			})
		}
	})

	When("BSL configuration changes while Velero is not running", func() {
		It("Should invalidate and recover backup repositories on startup", func() {
			oneHourTimeout, ctxCancel := context.WithTimeout(context.Background(), time.Minute*60)
			defer ctxCancel()

			backupName := "backup-" + UUIDgen.String()
			backupLocation := "default"

			By("Create namespace for sample workload", func() {
				Expect(CreateNamespace(oneHourTimeout, *veleroCfg.ClientToInstallVelero, startupValidationTestNs)).To(Succeed())
			})

			By("Deploy sample workload of Kibishii", func() {
				Expect(KibishiiPrepareBeforeBackup(
					oneHourTimeout,
					*veleroCfg.ClientToInstallVelero,
					veleroCfg.CloudProvider,
					startupValidationTestNs,
					veleroCfg.RegistryCredentialFile,
					veleroCfg.Features,
					veleroCfg.KibishiiDirectory,
					DefaultKibishiiData,
					veleroCfg.ImageRegistryProxy,
					veleroCfg.WorkerOS,
				)).To(Succeed())
			})

			var BackupCfg BackupConfig
			BackupCfg.BackupName = backupName
			BackupCfg.Namespace = startupValidationTestNs
			BackupCfg.BackupLocation = backupLocation
			BackupCfg.UseVolumeSnapshots = false
			BackupCfg.DefaultVolumesToFsBackup = true

			By("Backup sample workload to establish BackupRepository", func() {
				Expect(VeleroBackupNamespace(oneHourTimeout, veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, BackupCfg.BackupName, "")
					return "Fail to backup workload"
				})
			})

			By("Verify backup completed successfully", func() {
				Expect(WaitForBackupToBeCreated(context.Background(), backupName, 10*time.Minute, &veleroCfg)).To(Succeed())
			})

			By("Verify BackupRepository is created and ready", func() {
				Expect(BackupRepositoriesCountShouldBe(context.Background(),
					veleroCfg.VeleroNamespace, startupValidationTestNs+"-"+backupLocation, 1)).To(Succeed())

				// Get the BackupRepository and verify it's ready
				repoList := &velerov1api.BackupRepositoryList{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
					client.InNamespace(veleroCfg.VeleroNamespace))).To(Succeed())

				var targetRepo *velerov1api.BackupRepository
				for i := range repoList.Items {
					if repoList.Items[i].Spec.VolumeNamespace == startupValidationTestNs {
						targetRepo = &repoList.Items[i]
						break
					}
				}
				Expect(targetRepo).NotTo(BeNil(), "BackupRepository not found")
				Expect(targetRepo.Status.Phase).To(Equal(velerov1api.BackupRepositoryPhaseReady))
			})

			// Store original BSL prefix for modification
			originalBSL := &velerov1api.BackupStorageLocation{}
			Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
				types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: backupLocation},
				originalBSL)).To(Succeed())

			originalPrefix := ""
			if originalBSL.Spec.StorageType.ObjectStorage != nil {
				originalPrefix = originalBSL.Spec.StorageType.ObjectStorage.Prefix
			}

			By("Scale down Velero deployment to simulate shutdown", func() {
				deployment := &appsv1api.Deployment{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
					types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: "velero"},
					deployment)).To(Succeed())

				zero := int32(0)
				deployment.Spec.Replicas = &zero
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Update(oneHourTimeout,
					deployment)).To(Succeed())

				// Wait for deployment to scale down
				Eventually(func() int32 {
					d := &appsv1api.Deployment{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
						types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: "velero"},
						d)
					if err != nil {
						return -1
					}
					return d.Status.ReadyReplicas
				}, 2*time.Minute, 5*time.Second).Should(Equal(int32(0)))

				// Ensure no Velero pods are running
				Eventually(func() int {
					podList := &corev1api.PodList{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, podList,
						client.InNamespace(veleroCfg.VeleroNamespace),
						client.MatchingLabels{"deploy": "velero"})
					if err != nil {
						return -1
					}
					runningCount := 0
					for _, pod := range podList.Items {
						if pod.Status.Phase == corev1api.PodRunning {
							runningCount++
						}
					}
					fmt.Printf("Velero pods running: %d\n", runningCount)
					return runningCount
				}, 30*time.Second, 2*time.Second).Should(Equal(0), "Velero pods should be fully terminated")
			})

			By("Modify BSL configuration (change prefix)", func() {
				bsl := &velerov1api.BackupStorageLocation{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
					types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: backupLocation},
					bsl)).To(Succeed())

				if bsl.Spec.StorageType.ObjectStorage != nil {
					fmt.Printf("Original BSL prefix: %s\n", bsl.Spec.StorageType.ObjectStorage.Prefix)
					// Change the prefix to trigger validation failure
					bsl.Spec.StorageType.ObjectStorage.Prefix = originalPrefix + "-modified"
					fmt.Printf("Modified BSL prefix: %s\n", bsl.Spec.StorageType.ObjectStorage.Prefix)
				}

				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Update(oneHourTimeout,
					bsl)).To(Succeed())
				fmt.Println("BSL configuration successfully modified")
			})

			By("Scale up Velero deployment to simulate startup", func() {
				deployment := &appsv1api.Deployment{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
					types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: "velero"},
					deployment)).To(Succeed())

				one := int32(1)
				deployment.Spec.Replicas = &one
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Update(oneHourTimeout,
					deployment)).To(Succeed())

				// Wait for deployment to scale up
				Eventually(func() int32 {
					d := &appsv1api.Deployment{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
						types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: "velero"},
						d)
					if err != nil {
						return -1
					}
					return d.Status.ReadyReplicas
				}, 2*time.Minute, 5*time.Second).Should(Equal(int32(1)))

				// Ensure Velero pod is fully running and ready
				Eventually(func() bool {
					podList := &corev1api.PodList{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, podList,
						client.InNamespace(veleroCfg.VeleroNamespace),
						client.MatchingLabels{"deploy": "velero"})
					if err != nil || len(podList.Items) != 1 {
						return false
					}
					pod := podList.Items[0]
					if pod.Status.Phase != corev1api.PodRunning {
						return false
					}
					// Check all containers are ready
					for _, condition := range pod.Status.Conditions {
						if condition.Type == corev1api.PodConditionType("Ready") && condition.Status == corev1api.ConditionTrue {
							fmt.Println("Velero pod is fully ready")
							return true
						}
					}
					return false
				}, 1*time.Minute, 2*time.Second).Should(BeTrue(), "Velero pod should be ready")

				// Force a reconciliation by adding/updating a label on the repository
				// This ensures the startup validation runs
				repoList := &velerov1api.BackupRepositoryList{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
					client.InNamespace(veleroCfg.VeleroNamespace))).To(Succeed())

				foundRepo := false
				for i := range repoList.Items {
					if repoList.Items[i].Spec.VolumeNamespace == startupValidationTestNs {
						repo := &repoList.Items[i]
						fmt.Printf("Found repository %s to update, current annotations: %+v\n", repo.Name, repo.Annotations)
						if repo.Labels == nil {
							repo.Labels = make(map[string]string)
						}
						repo.Labels["test-trigger"] = "reconcile"
						err := veleroCfg.ClientToInstallVelero.Kubebuilder.Update(oneHourTimeout, repo)
						Expect(err).NotTo(HaveOccurred(), "Failed to update repository to trigger reconciliation")
						fmt.Printf("Successfully updated repository %s with trigger label\n", repo.Name)
						foundRepo = true
						break
					}
				}
				Expect(foundRepo).To(BeTrue(), "No repository found for namespace "+startupValidationTestNs)

				// Give controller time to run startup validation
				// The validation now runs synchronously on first reconciliation
				time.Sleep(10 * time.Second)
			})

			By("Verify BackupRepository recovers and becomes Ready with new BSL config", func() {
				// The controller first marks the repo as NotReady with BSL change message,
				// then on the next reconciliation it recovers the repository by connecting
				// with the new BSL configuration. The same repository is reused and its
				// BSL annotations are updated to reflect the new configuration.
				Eventually(func() velerov1api.BackupRepositoryPhase {
					repoList := &velerov1api.BackupRepositoryList{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
						client.InNamespace(veleroCfg.VeleroNamespace))
					if err != nil {
						fmt.Printf("Error listing repos: %v\n", err)
						return ""
					}

					for _, repo := range repoList.Items {
						if repo.Spec.VolumeNamespace == startupValidationTestNs {
							fmt.Printf("Repository %s: Phase=%s, Message=%s\n",
								repo.Name, repo.Status.Phase, repo.Status.Message)
							return repo.Status.Phase
						}
					}
					fmt.Println("Repository not found")
					return ""
				}, 3*time.Minute, 10*time.Second).Should(Equal(velerov1api.BackupRepositoryPhaseReady),
					"BackupRepository should recover and become Ready after BSL change")
			})

			By("Verify BackupRepository annotations are updated with new BSL config", func() {
				repoList := &velerov1api.BackupRepositoryList{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
					client.InNamespace(veleroCfg.VeleroNamespace))).To(Succeed())

				var targetRepo *velerov1api.BackupRepository
				for i := range repoList.Items {
					if repoList.Items[i].Spec.VolumeNamespace == startupValidationTestNs {
						targetRepo = &repoList.Items[i]
						break
					}
				}
				Expect(targetRepo).NotTo(BeNil(), "BackupRepository not found")
				Expect(targetRepo.Status.Phase).To(Equal(velerov1api.BackupRepositoryPhaseReady))

				// Verify the BSL prefix annotation was updated to the new value
				prefixAnnotation := targetRepo.Annotations[velerov1api.BSLPrefixAnnotation]
				fmt.Printf("Repository BSL prefix annotation: %s\n", prefixAnnotation)
				Expect(prefixAnnotation).To(ContainSubstring("-modified"),
					"BSL prefix annotation should reflect the modified prefix")
			})

			fmt.Printf("|| EXPECTED || - Backup repository startup validation test completed successfully\n")
		})
	})

	When("BSL configuration changes while Velero is running", func() {
		It("Should invalidate and recover backup repositories during runtime BSL changes", func() {
			oneHourTimeout, ctxCancel := context.WithTimeout(context.Background(), time.Minute*60)
			defer ctxCancel()

			backupName := "backup-runtime-" + UUIDgen.String()
			backupLocation := "default"

			By("Create namespace for sample workload", func() {
				Expect(CreateNamespace(oneHourTimeout, *veleroCfg.ClientToInstallVelero, startupValidationTestNs)).To(Succeed())
			})

			By("Deploy sample workload of Kibishii", func() {
				Expect(KibishiiPrepareBeforeBackup(
					oneHourTimeout,
					*veleroCfg.ClientToInstallVelero,
					veleroCfg.CloudProvider,
					startupValidationTestNs,
					veleroCfg.RegistryCredentialFile,
					veleroCfg.Features,
					veleroCfg.KibishiiDirectory,
					DefaultKibishiiData,
					veleroCfg.ImageRegistryProxy,
					veleroCfg.WorkerOS,
				)).To(Succeed())
			})

			var BackupCfg BackupConfig
			BackupCfg.BackupName = backupName
			BackupCfg.Namespace = startupValidationTestNs
			BackupCfg.BackupLocation = backupLocation
			BackupCfg.UseVolumeSnapshots = false
			BackupCfg.DefaultVolumesToFsBackup = true

			By("Backup sample workload to establish BackupRepository", func() {
				Expect(VeleroBackupNamespace(oneHourTimeout, veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, BackupCfg.BackupName, "")
					return "Fail to backup workload"
				})
			})

			By("Verify backup completed successfully", func() {
				Expect(WaitForBackupToBeCreated(context.Background(), backupName, 10*time.Minute, &veleroCfg)).To(Succeed())
			})

			By("Verify BackupRepository is created and ready", func() {
				Expect(BackupRepositoriesCountShouldBe(context.Background(),
					veleroCfg.VeleroNamespace, startupValidationTestNs+"-"+backupLocation, 1)).To(Succeed())

				repoList := &velerov1api.BackupRepositoryList{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
					client.InNamespace(veleroCfg.VeleroNamespace))).To(Succeed())

				var targetRepo *velerov1api.BackupRepository
				for i := range repoList.Items {
					if repoList.Items[i].Spec.VolumeNamespace == startupValidationTestNs {
						targetRepo = &repoList.Items[i]
						break
					}
				}
				Expect(targetRepo).NotTo(BeNil(), "BackupRepository not found")
				Expect(targetRepo.Status.Phase).To(Equal(velerov1api.BackupRepositoryPhaseReady))
			})

			// Store original BSL prefix for modification
			originalBSL := &velerov1api.BackupStorageLocation{}
			Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
				types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: backupLocation},
				originalBSL)).To(Succeed())

			originalPrefix := ""
			if originalBSL.Spec.StorageType.ObjectStorage != nil {
				originalPrefix = originalBSL.Spec.StorageType.ObjectStorage.Prefix
			}

			By("Modify BSL configuration while Velero is running (change prefix)", func() {
				bsl := &velerov1api.BackupStorageLocation{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Get(oneHourTimeout,
					types.NamespacedName{Namespace: veleroCfg.VeleroNamespace, Name: backupLocation},
					bsl)).To(Succeed())

				if bsl.Spec.StorageType.ObjectStorage != nil {
					fmt.Printf("Original BSL prefix: %s\n", bsl.Spec.StorageType.ObjectStorage.Prefix)
					bsl.Spec.StorageType.ObjectStorage.Prefix = originalPrefix + "-runtime-modified"
					fmt.Printf("Modified BSL prefix: %s\n", bsl.Spec.StorageType.ObjectStorage.Prefix)
				}

				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.Update(oneHourTimeout,
					bsl)).To(Succeed())
				fmt.Println("BSL configuration successfully modified while Velero is running")
			})

			By("Verify BackupRepository recovers and becomes Ready after runtime BSL change", func() {
				// The BSL watcher detects the change, marks repo NotReady,
				// then the controller recovers it with the new BSL configuration.
				// The same repository is reused and its annotations are updated.
				Eventually(func() velerov1api.BackupRepositoryPhase {
					repoList := &velerov1api.BackupRepositoryList{}
					err := veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
						client.InNamespace(veleroCfg.VeleroNamespace))
					if err != nil {
						fmt.Printf("Error listing repos: %v\n", err)
						return ""
					}

					for _, repo := range repoList.Items {
						if repo.Spec.VolumeNamespace == startupValidationTestNs {
							fmt.Printf("Repository %s: Phase=%s, Message=%s\n",
								repo.Name, repo.Status.Phase, repo.Status.Message)
							return repo.Status.Phase
						}
					}
					fmt.Println("Repository not found")
					return ""
				}, 3*time.Minute, 10*time.Second).Should(Equal(velerov1api.BackupRepositoryPhaseReady),
					"BackupRepository should recover and become Ready after runtime BSL change")
			})

			By("Verify BackupRepository annotations are updated with new BSL config", func() {
				repoList := &velerov1api.BackupRepositoryList{}
				Expect(veleroCfg.ClientToInstallVelero.Kubebuilder.List(oneHourTimeout, repoList,
					client.InNamespace(veleroCfg.VeleroNamespace))).To(Succeed())

				var targetRepo *velerov1api.BackupRepository
				for i := range repoList.Items {
					if repoList.Items[i].Spec.VolumeNamespace == startupValidationTestNs {
						targetRepo = &repoList.Items[i]
						break
					}
				}
				Expect(targetRepo).NotTo(BeNil(), "BackupRepository not found")
				Expect(targetRepo.Status.Phase).To(Equal(velerov1api.BackupRepositoryPhaseReady))

				// Verify the BSL prefix annotation was updated to the new value
				prefixAnnotation := targetRepo.Annotations[velerov1api.BSLPrefixAnnotation]
				fmt.Printf("Repository BSL prefix annotation: %s\n", prefixAnnotation)
				Expect(prefixAnnotation).To(ContainSubstring("-runtime-modified"),
					"BSL prefix annotation should reflect the runtime-modified prefix")
			})

			fmt.Printf("|| EXPECTED || - Backup repository runtime validation test completed successfully\n")
		})
	})
}
