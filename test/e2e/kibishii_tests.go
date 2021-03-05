package e2e

import (
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

const (
	kibishiiNamespace = "kibishii-workload"
	jumpPadPod        = "jump-pad"
)

func installKibishii(ctx context.Context, namespace string, cloudPlatform string) error {
	// We use kustomize to generate YAML for Kibishii from the checked-in yaml directories
	kibishiiInstallCmd := exec.CommandContext(ctx, "kubectl", "apply", "-n", namespace, "-k",
		"github.com/vmware-tanzu-experiments/distributed-data-generator/kubernetes/yaml/"+cloudPlatform)

	_, _, err := veleroexec.RunCommand(kibishiiInstallCmd)
	if err != nil {
		return errors.Wrap(err, "failed to install kibishii")
	}

	kibishiiSetWaitCmd := exec.CommandContext(ctx, "kubectl", "rollout", "status", "statefulset.apps/kibishii-deployment",
		"-n", namespace, "-w", "--timeout=30m")
	_, _, err = veleroexec.RunCommand(kibishiiSetWaitCmd)

	if err != nil {
		return err
	}

	fmt.Printf("Waiting for kibishii jump-pad pod to be ready\n")
	jumpPadWaitCmd := exec.CommandContext(ctx, "kubectl", "wait", "--for=condition=ready", "-n", namespace, "pod/jump-pad")
	_, _, err = veleroexec.RunCommand(jumpPadWaitCmd)
	if err != nil {
		return errors.Wrapf(err, "Failed to wait for ready status of pod %s/%s", namespace, jumpPadPod)
	}

	// TODO - Fix kibishii so we can check that it is ready to go
	// Wait for etcd run as kibishii workload to be ready
	fmt.Printf("Waiting for etcd workload pods to be ready\n")
	exec.CommandContext(ctx, "kubectl", "wait", "--for=condition=ready", "pod/etcd0", "pod/etcd1", "pod/etcd2")

	return err
}

func generateData(ctx context.Context, namespace string, levels int, filesPerLevel int, dirsPerLevel int, fileSize int,
	blockSize int, passNum int, expectedNodes int) error {
	kibishiiGenerateCmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, "jump-pad", "--",
		"/usr/local/bin/generate.sh", strconv.Itoa(levels), strconv.Itoa(filesPerLevel), strconv.Itoa(dirsPerLevel), strconv.Itoa(fileSize),
		strconv.Itoa(blockSize), strconv.Itoa(passNum), strconv.Itoa(expectedNodes))
	fmt.Printf("kibishiiGenerateCmd cmd =%v\n", kibishiiGenerateCmd)

	_, _, err := veleroexec.RunCommand(kibishiiGenerateCmd)
	if err != nil {
		return errors.Wrap(err, "failed to generate")
	}

	return nil
}

func verifyData(ctx context.Context, namespace string, levels int, filesPerLevel int, dirsPerLevel int, fileSize int,
	blockSize int, passNum int, expectedNodes int) error {
	kibishiiVerifyCmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, "jump-pad", "--",
		"/usr/local/bin/verify.sh", strconv.Itoa(levels), strconv.Itoa(filesPerLevel), strconv.Itoa(dirsPerLevel), strconv.Itoa(fileSize),
		strconv.Itoa(blockSize), strconv.Itoa(passNum), strconv.Itoa(expectedNodes))
	fmt.Printf("kibishiiVerifyCmd cmd =%v\n", kibishiiVerifyCmd)

	_, _, err := veleroexec.RunCommand(kibishiiVerifyCmd)
	if err != nil {
		return errors.Wrap(err, "failed to verify")
	}
	return nil
}

// RunKibishiiTests runs kibishii tests on the provider.
func RunKibishiiTests(client *kubernetes.Clientset, providerName, veleroCLI, veleroNamespace, backupName, restoreName string) error {
	fiveMinTimeout, _ := context.WithTimeout(context.Background(), 5*time.Minute)
	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)
	timeout := 10 * time.Minute
	interval := 5 * time.Second

	if err := CreateNamespace(fiveMinTimeout, client, kibishiiNamespace); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", kibishiiNamespace)
	}

	if err := installKibishii(fiveMinTimeout, kibishiiNamespace, providerName); err != nil {
		return errors.Wrap(err, "Failed to install Kibishii workload")
	}

	if err := generateData(oneHourTimeout, kibishiiNamespace, 2, 10, 10, 1024, 1024, 0, 2); err != nil {
		return errors.Wrap(err, "Failed to generate data")
	}

	if err := VeleroBackupNamespace(oneHourTimeout, veleroCLI, veleroNamespace, backupName, kibishiiNamespace); err != nil {
		VeleroBackupLogs(fiveMinTimeout, veleroCLI, veleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", kibishiiNamespace)
	}

	fmt.Printf("Simulating a disaster by removing namespace %s\n", kibishiiNamespace)
	if err := client.CoreV1().Namespaces().Delete(oneHourTimeout, kibishiiNamespace, metav1.DeleteOptions{}); err != nil {
		return errors.Wrap(err, "Failed to simulate a disaster")
	}
	// wait for ns delete
	err := WaitForNamespaceDeletion(interval, timeout, client, kibishiiNamespace)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Failed to wait for deletion of namespace %s", kibishiiNamespace))
	}

	if err := VeleroRestore(oneHourTimeout, veleroCLI, veleroNamespace, restoreName, backupName); err != nil {
		VeleroRestoreLogs(fiveMinTimeout, veleroCLI, veleroNamespace, restoreName)
		return errors.Wrapf(err, "Restore %s failed from backup %s", restoreName, backupName)
	}

	// wait for kibishii pod startup
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		kp, err := client.CoreV1().Pods(kibishiiNamespace).Get(context.TODO(), jumpPadPod, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, fmt.Sprintf("Failed to verify pod %s/%s is %s", kibishiiNamespace, jumpPadPod, corev1api.PodRunning))
		}
		if kp.Status.Phase != corev1api.PodRunning {
			fmt.Printf("Pod %s is in state %s waiting for it to be %s\n", jumpPadPod, kp.Status.Phase, corev1api.PodRunning)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Failed to wait for pod %s/%s to start running", kibishiiNamespace, jumpPadPod))
	}

	// TODO - check that namespace exists
	fmt.Printf("running kibishii verify\n")
	if err := verifyData(oneHourTimeout, kibishiiNamespace, 2, 10, 10, 1024, 1024, 0, 2); err != nil {
		return errors.Wrap(err, "Failed to verify data generated by kibishii")
	}

	if err := client.CoreV1().Namespaces().Delete(oneHourTimeout, kibishiiNamespace, metav1.DeleteOptions{}); err != nil {
		return errors.Wrapf(err, "Failed to cleanup %s wrokload namespace", kibishiiNamespace)
	}
	// wait for ns delete
	err = WaitForNamespaceDeletion(interval, timeout, client, kibishiiNamespace)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Failed to wait for deletion of namespace %s", kibishiiNamespace))
	}
	fmt.Printf("kibishii test completed successfully\n")
	return nil
}
