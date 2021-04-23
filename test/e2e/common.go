package e2e

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	corev1api "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

// ensureClusterExists returns whether or not a kubernetes cluster exists for tests to be run on.
func ensureClusterExists(ctx context.Context) error {
	return exec.CommandContext(ctx, "kubectl", "cluster-info").Run()
}

// createNamespace creates a kubernetes namespace
func createNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) error {
	ns := builder.ForNamespace(namespace).Result()
	_, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// waitForNamespaceDeletion waits for namespace to be deleted.
func waitForNamespaceDeletion(interval, timeout time.Duration, client *kubernetes.Clientset, ns string) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		fmt.Printf("Namespace %s is still being deleted...\n", ns)
		return false, nil
	})
	return err
}

func createSecretFromFiles(ctx context.Context, client *kubernetes.Clientset, namespace string, name string, files map[string]string) error {
	data := make(map[string][]byte)

	for key, filePath := range files {
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.WithMessagef(err, "Failed to read secret file %q", filePath)
		}

		data[key] = contents
	}

	secret := builder.ForSecret(namespace, name).Data(data).Result()
	_, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// waitForPods waits until all of the pods have gone to PodRunning state
func waitForPods(ctx context.Context, client *kubernetes.Clientset, namespace string, pods []string) error {
	timeout := 10 * time.Minute
	interval := 5 * time.Second
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		for _, podName := range pods {
			checkPod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false, errors.WithMessage(err, fmt.Sprintf("Failed to verify pod %s/%s is %s", namespace, podName, corev1api.PodRunning))
			}
			// If any pod is still waiting we don't need to check any more so return and wait for next poll interval
			if checkPod.Status.Phase != corev1api.PodRunning {
				fmt.Printf("Pod %s is in state %s waiting for it to be %s\n", podName, checkPod.Status.Phase, corev1api.PodRunning)
				return false, nil
			}
		}
		// All pods were in PodRunning state, we're successful
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Failed to wait for pods in namespace %s to start running", namespace))
	}
	return nil
}
