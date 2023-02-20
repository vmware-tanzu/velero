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
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware-tanzu/velero/pkg/builder"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

var veleroCfg VeleroConfig

func APIExtensionsVersionsTest() {
	var (
		backupName, restoreName string
	)

	resourceName := "apiextensions.k8s.io"
	crdName := "rocknrollbands.music.example.io"
	label := "for=backup"
	srcCrdYaml := "testdata/enable_api_group_versions/case-a-source-v1beta1.yaml"
	BeforeEach(func() {
		if veleroCfg.DefaultCluster == "" && veleroCfg.StandbyCluster == "" {
			Skip("CRD with apiextension versions migration test needs 2 clusters")
		}
		veleroCfg = VeleroCfg
		Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
		srcVersions, err := GetAPIVersions(veleroCfg.DefaultClient, resourceName)
		Expect(err).ShouldNot(HaveOccurred())
		dstVersions, err := GetAPIVersions(veleroCfg.StandbyClient, resourceName)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(srcVersions).Should(ContainElement("v1"), func() string {
			Skip("CRD with apiextension versions srcVersions should have v1")
			return ""
		})
		Expect(srcVersions).Should(ContainElement("v1beta1"), func() string {
			Skip("CRD with apiextension versions srcVersions should have v1")
			return ""
		})
		Expect(dstVersions).Should(ContainElement("v1"), func() string {
			Skip("CRD with apiextension versions dstVersions should have v1")
			return ""
		})
		Expect(len(srcVersions) > 1 && len(dstVersions) == 1).Should(Equal(true), func() string {
			Skip("Source cluster should support apiextension v1 and v1beta1, destination cluster should only support apiextension v1")
			return ""
		})
	})
	AfterEach(func() {
		if !veleroCfg.Debug {
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *veleroCfg.DefaultClient)
			})
			if veleroCfg.InstallVelero {
				By("Uninstall Velero and delete CRD ", func() {
					Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace)).To(Succeed())
					Expect(deleteCRDByName(context.Background(), crdName)).To(Succeed())

					Expect(KubectlConfigUseContext(context.Background(), veleroCfg.StandbyCluster)).To(Succeed())
					Expect(VeleroUninstall(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace)).To(Succeed())
					Expect(deleteCRDByName(context.Background(), crdName)).To(Succeed())
				})
			}
			By(fmt.Sprintf("Switch to default kubeconfig context %s", veleroCfg.DefaultCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
				veleroCfg.ClientToInstallVelero = veleroCfg.DefaultClient
			})
		}

	})
	Context("When EnableAPIGroupVersions flag is set", func() {
		It("Enable API Group to B/R CRD APIExtensionsVersions", func() {
			backupName = "backup-" + UUIDgen.String()
			restoreName = "restore-" + UUIDgen.String()

			By(fmt.Sprintf("Install Velero in cluster-A (%s) to backup workload", veleroCfg.DefaultCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.DefaultCluster)).To(Succeed())
				veleroCfg.Features = "EnableAPIGroupVersions"
				veleroCfg.UseVolumeSnapshots = false
				Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
			})

			By(fmt.Sprintf("Install CRD of apiextenstions v1beta1 in cluster-A (%s)", veleroCfg.DefaultCluster), func() {
				Expect(installCRD(context.Background(), srcCrdYaml)).To(Succeed())
				Expect(CRDShouldExist(context.Background(), crdName)).To(Succeed())
				Expect(WaitForCRDEstablished(crdName)).To(Succeed())
				Expect(AddLabelToCRD(context.Background(), crdName, label)).To(Succeed())
				// Velero server refresh api version data by discovery helper every 5 minutes
				time.Sleep(6 * time.Minute)
			})

			By("Backup CRD", func() {
				var BackupCfg BackupConfig
				BackupCfg.BackupName = backupName
				BackupCfg.IncludeResources = "crd"
				BackupCfg.IncludeClusterResources = true
				BackupCfg.Selector = label
				Expect(VeleroBackupNamespace(context.Background(), veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace, backupName, "")
					return "Fail to backup workload"
				})
			})

			By(fmt.Sprintf("Install Velero in cluster-B (%s) to restore workload", veleroCfg.StandbyCluster), func() {
				Expect(KubectlConfigUseContext(context.Background(), veleroCfg.StandbyCluster)).To(Succeed())
				veleroCfg.ClientToInstallVelero = veleroCfg.StandbyClient
				Expect(VeleroInstall(context.Background(), &veleroCfg)).To(Succeed())
			})

			By(fmt.Sprintf("Waiting for backups sync to Velero in cluster-B (%s)", veleroCfg.StandbyCluster), func() {
				Expect(WaitForBackupToBeCreated(context.Background(), veleroCfg.VeleroCLI, backupName, 5*time.Minute)).To(Succeed())
			})

			By(fmt.Sprintf("CRD %s should not exist in cluster-B (%s)", crdName, veleroCfg.StandbyCluster), func() {
				Expect(CRDShouldNotExist(context.Background(), crdName)).To(Succeed(), "Error: CRD already exists in cluster B, clean it and re-run test")
			})

			By("Restore CRD", func() {
				Expect(VeleroRestore(context.Background(), veleroCfg.VeleroCLI,
					veleroCfg.VeleroNamespace, restoreName, backupName, "")).To(Succeed(), func() string {
					RunDebug(context.Background(), veleroCfg.VeleroCLI,
						veleroCfg.VeleroNamespace, "", restoreName)
					return "Fail to restore workload"
				})
			})

			By("Verify CRD restore ", func() {
				Expect(CRDShouldExist(context.Background(), crdName)).To(Succeed())
			})
		})
	})
}
func APIGropuVersionsTest() {
	var (
		resource, group string
		err             error
		ctx             = context.Background()
	)

	BeforeEach(func() {
		veleroCfg = VeleroCfg
		resource = "rockbands"
		group = "music.example.io"
		UUIDgen, err = uuid.NewRandom()
		Expect(err).NotTo(HaveOccurred())
		flag.Parse()
		// TODO: install Velero once for the test suite once feature flag is
		// removed and velero installation becomes the same as other e2e tests.
		if veleroCfg.InstallVelero {
			veleroCfg.Features = "EnableAPIGroupVersions"
			veleroCfg.UseVolumeSnapshots = false
			err = VeleroInstall(context.Background(), &veleroCfg)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		if !veleroCfg.Debug {
			fmt.Printf("Clean up resource: kubectl delete crd %s.%s\n", resource, group)
			cmd := exec.CommandContext(ctx, "kubectl", "delete", "crd", resource+"."+group)
			_, stderr, err := veleroexec.RunCommand(cmd)
			if strings.Contains(stderr, "NotFound") {
				fmt.Printf("Ignore error: %v\n", stderr)
				err = nil
			}
			Expect(err).NotTo(HaveOccurred())
			By("Clean backups after test", func() {
				DeleteBackups(context.Background(), *veleroCfg.ClientToInstallVelero)
			})
			if veleroCfg.InstallVelero {

				By("Uninstall Velero", func() {
					Expect(VeleroUninstall(ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace)).NotTo(HaveOccurred())
				})
			}
		}
	})

	Context("When EnableAPIGroupVersions flag is set", func() {
		It("Should back up API group version and restore by version priority", func() {
			Expect(runEnableAPIGroupVersionsTests(
				ctx,
				*veleroCfg.ClientToInstallVelero,
				resource,
				group,
			)).To(Succeed(), "Failed to successfully backup and restore multiple API Groups")
		})
	})
}

func runEnableAPIGroupVersionsTests(ctx context.Context, client TestClient, resource, group string) error {
	tests := []struct {
		name       string
		namespaces []string
		srcCrdYaml string
		srcCRs     map[string]string
		tgtCrdYaml string
		tgtVer     string
		cm         *corev1api.ConfigMap
		gvs        map[string][]string
		want       map[string]map[string]string
	}{
		{
			name:       "Target and source cluster preferred versions match; Preferred version v1 is restored (Priority 1, Case A).",
			srcCrdYaml: "testdata/enable_api_group_versions/case-a-source.yaml",
			srcCRs: map[string]string{
				"v1":       "testdata/enable_api_group_versions/music_v1_rockband.yaml",
				"v1alpha1": "testdata/enable_api_group_versions/music_v1alpha1_rockband.yaml",
			},
			tgtCrdYaml: "testdata/enable_api_group_versions/case-a-target.yaml",
			tgtVer:     "v1",
			cm:         nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v1",
				},
				"specs": {
					"genre": "60s rock",
				},
			},
		},
		{
			name:       "Latest common non-preferred supported version v2beta2 is restored (Priority 3, Case D).",
			srcCrdYaml: "testdata/enable_api_group_versions/case-b-source-manually-added-mutations.yaml",
			srcCRs: map[string]string{
				"v2beta2": "testdata/enable_api_group_versions/music_v2beta2_rockband.yaml",
				"v2beta1": "testdata/enable_api_group_versions/music_v2beta1_rockband.yaml",
				"v1":      "testdata/enable_api_group_versions/music_v1_rockband.yaml",
			},
			tgtCrdYaml: "testdata/enable_api_group_versions/case-d-target-manually-added-mutations.yaml",
			tgtVer:     "v2beta2",
			cm:         nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v2beta2",
				},
				"specs": {
					"genre": "60s rock",
				},
			},
		},
		{
			name:       "No common supported versions means no rockbands custom resource is restored.",
			srcCrdYaml: "testdata/enable_api_group_versions/case-a-source.yaml",
			srcCRs: map[string]string{
				"v1":       "testdata/enable_api_group_versions/music_v1_rockband.yaml",
				"v1alpha1": "testdata/enable_api_group_versions/music_v1alpha1_rockband.yaml",
			},
			tgtCrdYaml: "testdata/enable_api_group_versions/case-b-target-manually-added-mutations.yaml",
			tgtVer:     "",
			cm:         nil,
			want:       nil,
		},
		{
			name:       "User config map overrides Priority 3, Case D and restores v2beta1",
			srcCrdYaml: "testdata/enable_api_group_versions/case-b-source-manually-added-mutations.yaml",
			srcCRs: map[string]string{
				"v2beta2": "testdata/enable_api_group_versions/music_v2beta2_rockband.yaml",
				"v2beta1": "testdata/enable_api_group_versions/music_v2beta1_rockband.yaml",
				"v1":      "testdata/enable_api_group_versions/music_v1_rockband.yaml",
			},
			tgtCrdYaml: "testdata/enable_api_group_versions/case-d-target-manually-added-mutations.yaml",
			tgtVer:     "v2beta1",
			cm: builder.ForConfigMap(veleroCfg.VeleroNamespace, "enableapigroupversions").Data(
				"restoreResourcesVersionPriority",
				`rockbands.music.example.io=v2beta1,v2beta2,v2`,
			).Result(),
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v2beta1",
				},
				"specs": {
					"genre": "60s rock",
				},
			},
		},
		{
			name:       "Restore successful when CRD doesn't (yet) exist in target",
			srcCrdYaml: "testdata/enable_api_group_versions/case-a-source.yaml",
			srcCRs: map[string]string{
				"v1": "testdata/enable_api_group_versions/music_v1_rockband.yaml",
			},
			tgtCrdYaml: "",
			tgtVer:     "v1",
			cm:         nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v1",
				},
				"specs": {
					"genre": "60s rock",
				},
			},
		},
	}

	for i, tc := range tests {
		fmt.Printf("\n====== Test Case %d: %s ======\n", i, tc.name)

		err := installCRD(ctx, tc.srcCrdYaml)
		if err != nil {
			return errors.Wrap(err, "install music-system CRD on source cluster")
		}

		for version, cr := range tc.srcCRs {
			ns := resource + "-src-" + version

			if err := CreateNamespace(ctx, client, ns); err != nil {
				return errors.Wrapf(err, "create %s namespace", ns)
			}
			defer func(namespace string) {
				if err = DeleteNamespace(ctx, client, namespace, true); err != nil {
					fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", ns))
				}
			}(ns)

			if err := installCR(ctx, cr, ns); err != nil {
				return errors.Wrapf(err, "install %s custom resource on source cluster in namespace %s", cr, ns)
			}

			tc.namespaces = append(tc.namespaces, ns)
		}

		// Velero server refresh api version data by discovery helper every 5 minutes
		time.Sleep(6 * time.Minute)

		backup := "backup-rockbands-" + UUIDgen.String() + "-" + strconv.Itoa(i)
		namespacesStr := strings.Join(tc.namespaces, ",")

		var BackupCfg BackupConfig
		BackupCfg.BackupName = backup
		BackupCfg.Namespace = namespacesStr
		BackupCfg.BackupLocation = ""
		BackupCfg.UseVolumeSnapshots = false
		BackupCfg.Selector = ""

		Expect(VeleroBackupNamespace(ctx, veleroCfg.VeleroCLI,
			veleroCfg.VeleroNamespace, BackupCfg)).To(Succeed(), func() string {
			RunDebug(context.Background(), veleroCfg.VeleroCLI,
				veleroCfg.VeleroNamespace, backup, "")
			return "Fail to backup workload"
		})

		if err := deleteCRD(ctx, tc.srcCrdYaml); err != nil {
			return errors.Wrapf(err, "delete music-system CRD from source cluster")
		}

		for _, ns := range tc.namespaces {
			if err := DeleteNamespace(ctx, client, ns, true); err != nil {
				return errors.Wrapf(err, "delete %s namespace from source cluster", ns)
			}
		}

		// Install music-system CRD for target cluster.
		if tc.tgtCrdYaml != "" {
			if err := installCRD(ctx, tc.tgtCrdYaml); err != nil {
				return errors.Wrapf(err, "install music-system CRD on target cluster")
			}
		}

		// Apply config map if there is one.
		if tc.cm != nil {
			_, err := client.ClientGo.CoreV1().ConfigMaps(veleroCfg.VeleroNamespace).Create(ctx, tc.cm, metav1.CreateOptions{})
			if err != nil {
				return errors.Wrap(err, "create config map with user version priorities")
			}
		}

		// Velero server refresh api version data by discovery helper every 5 minutes
		time.Sleep(6 * time.Minute)

		// Restore rockbands namespaces.
		restore := "restore-rockbands-" + UUIDgen.String() + "-" + strconv.Itoa(i)

		if tc.want != nil {
			if err := VeleroRestore(ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, restore, backup, ""); err != nil {
				RunDebug(context.Background(), veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, "", restore)
				return errors.Wrapf(err, "restore %s namespaces on target cluster", namespacesStr)
			}

			annoSpec, err := resourceInfo(ctx, group, tc.tgtVer, resource)
			if err != nil {
				return errors.Wrapf(
					err,
					"get annotation and spec from %s.%s/%s object",
					resource,
					group,
					tc.tgtVer,
				)
			}

			// Assertion
			if !containsAll(annoSpec["annotations"], tc.want["annotations"]) {
				msg := fmt.Sprintf(
					"actual annotations: %v, expected annotations: %v",
					annoSpec["annotations"],
					tc.want["annotations"],
				)
				return errors.New(msg)
			}

			// Assertion
			if !containsAll(annoSpec["specs"], tc.want["specs"]) {
				msg := fmt.Sprintf(
					"actual specs: %v, expected specs: %v",
					annoSpec["specs"],
					tc.want["specs"],
				)
				return errors.New(msg)
			}

		} else {
			// No custom resource should have been restored. Expect "no resource found"
			// error during restore.
			err := VeleroRestore(ctx, veleroCfg.VeleroCLI, veleroCfg.VeleroNamespace, restore, backup, "")
			if !strings.Contains(err.Error(), "Unexpected restore phase got PartiallyFailed, expecting Completed") {
				return errors.New("expected error but not none")
			}
		}

		_ = deleteCRD(ctx, tc.srcCrdYaml)
		if tc.tgtCrdYaml != "" {
			_ = deleteCRD(ctx, tc.tgtCrdYaml)
		}
	}

	return nil
}

func installCRD(ctx context.Context, yaml string) error {
	fmt.Printf("Install CRD with %s.\n", yaml)
	err := KubectlApplyByFile(ctx, yaml)
	return err
}

func deleteCRD(ctx context.Context, yaml string) error {
	fmt.Println("Delete CRD", yaml)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", yaml, "--wait")

	_, stderr, err := veleroexec.RunCommand(cmd)
	if strings.Contains(stderr, "not found") {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, stderr)
	}

	return nil
}

func deleteCRDByName(ctx context.Context, name string) error {
	fmt.Println("Delete CRD", name)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "crd", name, "--wait")

	_, stderr, err := veleroexec.RunCommand(cmd)
	if strings.Contains(stderr, "not found") {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, stderr)
	}

	return nil
}

func installCR(ctx context.Context, crFile, ns string) error {
	retries := 5
	var stderr string
	var err error

	for i := 0; i < retries; i++ {
		fmt.Printf("Attempt %d: Install custom resource %s\n", i+1, crFile)
		cmd := exec.CommandContext(ctx, "kubectl", "apply", "-n", ns, "-f", crFile)
		_, stderr, err = veleroexec.RunCommand(cmd)
		if err == nil {
			fmt.Printf("Successfully installed CR on %s.\n", ns)
			return nil
		}

		fmt.Printf("Sleep for %ds before next attempt.\n", 20*i)
		time.Sleep(time.Second * time.Duration(i) * 20)
	}
	return errors.Wrap(err, stderr)
}

func resourceInfo(ctx context.Context, g, v, r string) (map[string]map[string]string, error) {
	rvg := r + "." + v + "." + g
	ns := r + "-src-" + v
	cmd := exec.CommandContext(ctx, "kubectl", "get", rvg, "-n", ns, "-o", "json")

	stdout, errMsg, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return nil, errors.Wrap(err, "unmarshal resource info JSON")
	}
	items := info["items"].([]interface{})

	if len(items) < 1 {
		return nil, errors.New("resource info is empty")
	}

	item := items[0].(map[string]interface{})
	metadata := item["metadata"].(map[string]interface{})
	annotations := metadata["annotations"].(map[string]interface{})
	specs := item["spec"].(map[string]interface{})

	annoSpec := make(map[string]map[string]string)

	for k, v := range annotations {
		if annoSpec["annotations"] == nil {
			annoSpec["annotations"] = map[string]string{
				k: v.(string),
			}
		} else {
			annoSpec["annotations"][k] = v.(string)
		}
	}

	for k, v := range specs {
		if val, ok := v.(string); ok {
			if annoSpec["specs"] == nil {
				annoSpec["specs"] = map[string]string{
					k: val,
				}
			} else {
				annoSpec["specs"][k] = val
			}
		}
	}

	return annoSpec, nil
}

// containsAll returns true if all the map values in the needles argument
// are found in the haystack argument values.
func containsAll(haystack, needles map[string]string) bool {
	for nkey, nval := range needles {

		hval, ok := haystack[nkey]
		if !ok {
			return false
		}

		if hval != nval {
			return false
		}
	}
	return true
}
