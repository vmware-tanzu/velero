package e2etestify

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/test/e2e"
)

func installVelero(ctx context.Context) error {
	if err := e2e.EnsureClusterExists(ctx); err != nil {
		return errors.Wrap(err, "check cluster exists")
	}

	// Pass global variables to option parameters.
	options, err := e2e.GetProviderVeleroInstallOptions(
		*pluginProvider,
		*cloudCredentialsFile,
		*bslBucket,
		*bslPrefix,
		*bslConfig,
		*vslConfig,
		providerPlugins(*pluginProvider),
	)
	if err != nil {
		return errors.Wrap(err, "get velero install options")
	}

	options.UseRestic = false
	options.Features = "EnableAPIGroupVersions"
	options.Image = *veleroImage

	if err := e2e.InstallVeleroServer(options); err != nil {
		// TODO pkg/install timeout has been increased from 1 to 3 mins. Is there a way
		// to configure that without hardcoding?
		return errors.Wrap(err, "install velero server")
	}

	return nil
}

func providerPlugins(provider string) []string {
	// TODO: make plugin images configurable
	switch provider {
	case "aws":
		return []string{"velero/velero-plugin-for-aws:v1.1.0"}
	case "azure":
		return []string{"velero/velero-plugin-for-microsoft-azure:v1.1.1"}
	case "vsphere":
		return []string{"velero/velero-plugin-for-aws:v1.1.0", "velero/velero-plugin-for-vsphere:v1.0.2"}
	default:
		return []string{""}
	}
}

func InstallCRD(ctx context.Context, crdFile, ns string) error {
	fmt.Printf("Install CRD %s.\n", crdFile)

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", crdFile)
	_, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		fmt.Println("Wait for CRD to be ready.")
		if err := WaitForPodContainers(ctx, ns); err != nil {
			return err
		}
	}

	return err
}

// PodContainersReady will get the pods and container status in a namespace.
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
		fmt.Println("CRD installation successful. Sleep for 20s for cluster to stabilize.")
		time.Sleep(time.Second * 20)
	}

	return err
}

func InstallCR(ctx context.Context, crFile, ns string) error {
	retries := 5
	var stderr string
	var err error

	// Using retries as "kubectl wait" depends on Condition, which not all CRs have.
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

func DeleteCRD(ctx context.Context, crdFile, ns string) error {
	fmt.Println("Delete CRD", crdFile)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", crdFile)

	_, stderr, err := veleroexec.RunCommand(cmd)
	if strings.Contains(stderr, "not found") {
		return nil
	}

	if err != nil {
		return errors.Wrap(err, stderr)
	}

	err = wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", ns)
		stdout, stderr, err := veleroexec.RunCommand(cmd)

		if strings.Contains(stderr, "not found") {
			return true, nil
		}

		if err != nil {
			return false, errors.Wrap(err, stderr)
		}

		re := regexp.MustCompile(ns)
		return re.MatchString(stdout), nil
	})

	return err
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
