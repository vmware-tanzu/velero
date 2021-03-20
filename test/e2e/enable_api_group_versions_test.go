package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/vmware-tanzu/velero/pkg/builder"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

var _ = Describe("[APIGroup] Velero tests with various CRD API group versions", func() {
	var (
		resource, group string
		certManagerCRD  map[string]resourceCRD
		err             error
		ctx             = context.Background()
	)
	const cert = "cert"

	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for group version tests")

	BeforeEach(func() {
		resource = "rockbands"
		group = "music.example.io"
		certManagerCRD =
			map[string]resourceCRD{
				cert: resourceCRD{
					url: "testdata/enable_api_group_versions/cert-manager.yaml",
					ns:  "cert-manager",
				},
			}

		err = InstallCRD(ctx, certManagerCRD[cert])
		Expect(err).NotTo(HaveOccurred())

		uuidgen, err = uuid.NewRandom()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "music-system")
		_, _, _ = veleroexec.RunCommand(cmd)

		cmd = exec.CommandContext(ctx, "kubectl", "delete", "crd", "rockbands.music.example.io")
		_, _, _ = veleroexec.RunCommand(cmd)

		_ = DeleteCRD(ctx, certManagerCRD[cert])
	})

	Context("When EnableAPIGroupVersions flag is set", func() {
		It("Should back up API group version and restore by version priority", func() {
			Expect(RunEnableAPIGroupVersionsTests(
				ctx,
				client,
				resource,
				group,
				certManagerCRD[cert],
			)).To(Succeed(), "Failed to successfully backup and restore multiple API Groups")
		})
	})
})

func RunEnableAPIGroupVersionsTests(ctx context.Context, client TestClient, resource, group string, certManager resourceCRD) error {
	const src = "source"
	const tgt = "target"

	tests := []struct {
		name       string
		namespaces []string
		resources  map[string]resourceCRD
		srcCRs     map[string]string
		tgtVer     string
		cm         *corev1api.ConfigMap
		gvs        map[string][]string
		want       map[string]map[string]string
	}{
		{
			name: "Target and source cluster preferred versions match; Preferred version v1 is restored (Priority 1, Case A).",
			resources: map[string]resourceCRD{
				src: resourceCRD{
					url: "testdata/enable_api_group_versions/case-a-source.yaml",
					ns:  "music-system",
				},
				tgt: resourceCRD{
					url: "testdata/enable_api_group_versions/case-a-target.yaml",
					ns:  "music-system",
				},
			},
			srcCRs: map[string]string{
				"v1":       "testdata/enable_api_group_versions/music_v1_rockband.yaml",
				"v1alpha1": "testdata/enable_api_group_versions/music_v1alpha1_rockband.yaml",
			},
			tgtVer: "v1",
			cm:     nil,
			want: map[string]map[string]string{

				"annotations": {
					"rockbands.music.example.io/originalVersion": "v1",
				},
				"specs": {
					"leadSinger": "John Lennon",
				},
			},
		},
		{
			name: "Latest common non-preferred supported version v2beta2 is restored (Priority 3, Case D).",
			resources: map[string]resourceCRD{
				src: resourceCRD{
					url: "testdata/enable_api_group_versions/case-b-source-manually-added-mutations.yaml",
					ns:  "music-system",
				},
				tgt: resourceCRD{
					url: "testdata/enable_api_group_versions/case-d-target-manually-added-mutations.yaml",
					ns:  "music-system",
				},
			},
			srcCRs: map[string]string{
				"v2beta2": "testdata/enable_api_group_versions/music_v2beta2_rockband.yaml",
				"v2beta1": "testdata/enable_api_group_versions/music_v2beta1_rockband.yaml",
				"v1":      "testdata/enable_api_group_versions/music_v1_rockband.yaml",
			},
			tgtVer: "v2beta2",
			cm:     nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v2beta2",
				},
				"specs": {
					"leadSinger": "John Lennon",
					"leadGuitar": "George Harrison",
					"drummer":    "Ringo Starr",
				},
			},
		},
		{
			name: "No common supported versions means no rockbands custom resource is restored.",
			resources: map[string]resourceCRD{
				src: resourceCRD{
					url: "testdata/enable_api_group_versions/case-a-source.yaml",
					ns:  "music-system",
				},
				tgt: resourceCRD{
					url: "testdata/enable_api_group_versions/case-b-target-manually-added-mutations.yaml",
					ns:  "music-system",
				},
			},
			srcCRs: map[string]string{
				"v1":       "testdata/enable_api_group_versions/music_v1_rockband.yaml",
				"v1alpha1": "testdata/enable_api_group_versions/music_v1alpha1_rockband.yaml",
			},
			tgtVer: "",
			cm:     nil,
			want:   nil,
		},
		{
			name: "User config map overrides Priority 3, Case D and restores v2beta1",
			resources: map[string]resourceCRD{
				src: resourceCRD{
					url: "testdata/enable_api_group_versions/case-b-source-manually-added-mutations.yaml",
					ns:  "music-system",
				},
				tgt: resourceCRD{
					url: "testdata/enable_api_group_versions/case-d-target-manually-added-mutations.yaml",
					ns:  "music-system",
				},
			},
			srcCRs: map[string]string{
				"v2beta2": "testdata/enable_api_group_versions/music_v2beta2_rockband.yaml",
				"v2beta1": "testdata/enable_api_group_versions/music_v2beta1_rockband.yaml",
				"v1":      "testdata/enable_api_group_versions/music_v1_rockband.yaml",
			},
			tgtVer: "v2beta1",
			cm: builder.ForConfigMap(veleroNamespace, "enableapigroupversions").Data(
				"restoreResourcesVersionPriority",
				`rockbands.music.example.io=v2beta1,v2beta2,v2`,
			).Result(),
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v2beta1",
				},
				"specs": {
					"leadSinger": "John Lennon",
					"leadGuitar": "George Harrison",
					"genre":      "60s rock",
				},
			},
		},
	}

	for i, tc := range tests {
		fmt.Printf("\n====== Test Case %d ======\n", i)

		err := InstallCRD(ctx, tc.resources[src])
		if err != nil {
			return errors.Wrap(err, "installing music-system CRD for source cluster")
		}

		for version, cr := range tc.srcCRs {
			ns := resource + "-src-" + version
			tc.namespaces = append(tc.namespaces, ns)

			if err := CreateNamespace(ctx, client, ns); err != nil {
				testCleanup(ctx, client, nil, []resourceCRD{
					certManager,
					tc.resources[src],
				})
				return errors.Wrapf(err, "creating %s namespace", ns)
			}

			if err := InstallCR(ctx, cr, ns); err != nil {
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
				})
				return errors.Wrapf(err, "installing %s custom resource on source cluster namespace %s", cr, ns)
			}

		}

		// TODO - Velero needs to be installed AFTER CRDs are installed because of https://github.com/vmware-tanzu/velero/issues/3471
		// Once that issue is fixed, we should install Velero once for the test suite
		if installVelero {
			VeleroInstall(context.Background(), veleroImage, veleroNamespace, cloudProvider, objectStoreProvider, false,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig,
				"EnableAPIGroupVersions" /* TODO - remove this when the feature flag is removed */)
			fmt.Println("Sleep 20s to wait for Velero to stabilize after install.")
			time.Sleep(time.Second * 20)
		}

		backup := "backup-rockbands-" + uuidgen.String() + "-" + strconv.Itoa(i)
		namespacesStr := strings.Join(tc.namespaces, ",")

		err = VeleroBackupNamespace(ctx, veleroCLI, veleroNamespace, backup, namespacesStr, "", false)
		if err != nil {
			VeleroBackupLogs(ctx, veleroCLI, veleroNamespace, backup)
			testCleanup(ctx, client, tc.namespaces, []resourceCRD{
				certManager,
				tc.resources[src],
			})
			return errors.Wrapf(err, "backing up %s namespaces on source cluster", namespacesStr)
		}

		// Delete music-system CRD and controllers installed on source cluster.
		if err := DeleteCRD(ctx, tc.resources[src]); err != nil {
			testCleanup(ctx, client, tc.namespaces, []resourceCRD{
				certManager,
			})
			return errors.Wrapf(err, "deleting music-system CRD from source cluster")
		}

		for _, ns := range tc.namespaces {
			if err := client.ClientGo.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
				testCleanup(ctx, client, nil, []resourceCRD{
					certManager,
					tc.resources[src],
				})
				return errors.Wrapf(err, "deleting %s namespace from source cluster", ns)
			}

			if err := WaitNamespaceDelete(ctx, ns); err != nil {
				return errors.Wrapf(err, "deleting %s namespace from source cluster", ns)
			}
		}

		// Install music-system CRD for target cluster.
		if err := InstallCRD(ctx, tc.resources[tgt]); err != nil {
			testCleanup(ctx, client, tc.namespaces, []resourceCRD{
				certManager,
				tc.resources[src],
			})
			return errors.Wrapf(err, "installing music-system CRD for target cluster")
		}

		// Apply config map if there is one.
		if tc.cm != nil {
			_, err := client.ClientGo.CoreV1().ConfigMaps(veleroNamespace).Create(ctx, tc.cm, metav1.CreateOptions{})
			if err != nil {
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.Wrap(err, "creating config map with user version priorities")
			}
		}

		// Reset Velero to recognize music-system CRD.
		if err := RestartPods(ctx, veleroNamespace); err != nil {
			testCleanup(ctx, client, tc.namespaces, []resourceCRD{
				certManager,
				tc.resources[src],
				tc.resources[tgt],
			})
			return errors.Wrapf(err, "restarting Velero pods")
		}
		fmt.Println("Sleep 20s to wait for Velero to stabilize after restart.")
		time.Sleep(time.Second * 20)

		// Restore rockbands namespace.
		restore := "restore-rockbands-" + uuidgen.String() + "-" + strconv.Itoa(i)

		if tc.want != nil {
			if err := VeleroRestore(ctx, veleroCLI, veleroNamespace, restore, backup); err != nil {
				VeleroRestoreLogs(ctx, veleroCLI, veleroNamespace, restore)
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.Wrapf(err, "restoring %s namespaces on target cluster", namespacesStr)
			}

			annoSpec, err := resourceInfo(ctx, group, tc.tgtVer, resource)
			if err != nil {
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.Wrapf(
					err,
					"get annotation and spec from %s.%s/%s object",
					resource,
					group,
					tc.tgtVer,
				)
			}

			// Assertion
			if containsAll(annoSpec["annotations"], tc.want["annotations"]) != true {
				msg := fmt.Sprintf(
					"actual annotations: %v, expected annotations: %v",
					annoSpec["annotations"],
					tc.want["annotations"],
				)
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.New(msg)
			}

			// Assertion
			if containsAll(annoSpec["specs"], tc.want["specs"]) != true {
				msg := fmt.Sprintf(
					"actual specs: %v, expected specs: %v",
					annoSpec["specs"],
					tc.want["specs"],
				)
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.New(msg)
			}
		} else {
			// No custom resource should have been restored. Expect "no resource found"
			// error during restore.
			err := VeleroRestore(ctx, veleroCLI, veleroNamespace, restore, backup)

			if err.Error() != "Unexpected restore phase got PartiallyFailed, expecting Completed" {
				testCleanup(ctx, client, tc.namespaces, []resourceCRD{
					certManager,
					tc.resources[src],
					tc.resources[tgt],
				})
				return errors.New("expected error but not none")
			}
		}

		testCleanup(ctx, client, tc.namespaces, []resourceCRD{
			certManager,
			tc.resources[src],
			tc.resources[tgt],
		})

		err = VeleroUninstall(client.Kubebuilder, installVelero, veleroNamespace)
		if err != nil {
			return err
		}
	}

	return nil
}

type resourceCRD struct {
	url, ns string
}

func testCleanup(ctx context.Context, client TestClient, namespaces []string, resources []resourceCRD) {
	// Delete namespaces created for CRs
	for _, ns := range namespaces {
		fmt.Println("Delete namespace", ns)
		_ = client.ClientGo.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
		_ = WaitNamespaceDelete(ctx, ns)
	}

	for _, rs := range resources {
		DeleteCRD(ctx, rs)
	}
}

func installVeleroForAPIGroups(ctx context.Context) error {
	if err := EnsureClusterExists(ctx); err != nil {
		return errors.Wrap(err, "check cluster exists")
	}

	// Pass global variables to option parameters.
	options, err := GetProviderVeleroInstallOptions(
		cloudProvider,
		cloudCredentialsFile,
		bslBucket,
		bslPrefix,
		bslConfig,
		vslConfig,
		getProviderPlugins(cloudProvider),
		"EnableAPIGroupVersions",
	)
	if err != nil {
		return errors.Wrap(err, "get velero install options")
	}

	options.UseRestic = false
	options.Features = "EnableAPIGroupVersions"
	options.Image = veleroImage

	if err := InstallVeleroServer(options); err != nil {
		return errors.WithMessagef(err, "Failed to install Velero in the cluster")
	}

	return nil
}

func InstallCRD(ctx context.Context, crd resourceCRD) error {
	fmt.Printf("Install CRD %s.\n", crd.url)

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", crd.url)
	_, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrap(err, stderr)
	}

	fmt.Println("Wait for CRD to be ready.")
	if err := WaitForPodContainers(ctx, crd.ns); err != nil {
		return err
	}

	return err
}

// WaitForPodContainers will get the pods and container status in a namespace.
// If the ratio of the number of containers running to total in a pod is not 1,
// it is not ready. Otherwise, if all container ratios are 1, the pod is running.
func WaitForPodContainers(ctx context.Context, ns string) error {
	err := wait.Poll(3*time.Second, 4*time.Minute, func() (bool, error) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", ns)
		stdout, stderr, err := veleroexec.RunCommand(cmd)

		if err != nil {
			return false, errors.Wrap(err, stderr)
		}

		re := regexp.MustCompile(`(\d)/(\d)\s+Running`)

		// Default allRunning needs to be false for when no match is found.
		var allRunning bool
		for i, v := range re.FindAllStringSubmatch(stdout, -1) {
			if i == 0 {
				allRunning = true
			}
			allRunning = v[1] == v[2] && allRunning
		}
		return allRunning, nil
	})

	if err == nil {
		fmt.Println("Sleep for 20s for cluster to stabilize.")
		time.Sleep(time.Second * 20)
	}

	return err
}

func DeleteCRD(ctx context.Context, crd resourceCRD) error {
	fmt.Println("Delete CRD", crd.url)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", crd.url, "--wait")

	_, stderr, err := veleroexec.RunCommand(cmd)
	if strings.Contains(stderr, "not found") {
		return nil
	}

	if err != nil {
		return errors.Wrap(err, stderr)
	}

	err = wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", crd.ns)
		stdout, stderr, err := veleroexec.RunCommand(cmd)

		if strings.Contains(stderr, "not found") {
			return true, nil
		}

		if err != nil {
			return false, errors.Wrap(err, stderr)
		}

		re := regexp.MustCompile(crd.ns)
		return re.MatchString(stdout), nil
	})
	return err
}

func RestartPods(ctx context.Context, ns string) error {
	fmt.Printf("Restart pods in %s namespace.\n", ns)

	cmd := exec.CommandContext(ctx, "kubectl", "delete", "pod", "--all", "-n", ns)
	_, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		fmt.Println("Wait for pods to be ready.")
		if err := WaitForPodContainers(ctx, ns); err != nil {
			return err
		}
	}

	return err
}

func InstallCR(ctx context.Context, crFile, ns string) error {
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

func WaitNamespaceDelete(ctx context.Context, ns string) error {
	err := wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", ns)

		stdout, stderr, err := veleroexec.RunCommand(cmd)
		if err != nil {
			return false, errors.Wrap(err, stderr)
		}

		re := regexp.MustCompile(ns)
		return re.MatchString(stdout), nil
	})

	return err
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
