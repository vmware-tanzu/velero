package e2e

import (
	"fmt"
	"os/exec"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

// EnsureClusterExists returns whether or not a kubernetes cluster exists for tests to be run on.
func EnsureClusterExists(ctx context.Context) error {
	return exec.CommandContext(ctx, "kubectl", "cluster-info").Run()
}

// GetClusterClient instantiates and returns a client for the cluster.
func GetClusterClient() (*kubernetes.Clientset, *apiextensionsclientset.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	extensionClientSet, err := apiextensionsclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return client, extensionClientSet, nil
}

// CreateNamespace creates a kubernetes namespace
func CreateNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) error {
	ns := builder.ForNamespace(namespace).Result()
	_, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func DeleteNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) error {
	err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		return errors.WithMessagef(err, "Delete namespace failed removing namespace %s", namespace)
	}
	return wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Namespaces.Get after delete return err %v\n", err)
			return true, nil // Assume any error means the delete was successful
		}
		return false, nil
	})
}
