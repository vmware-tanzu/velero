package e2e

import (
	"os/exec"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

const (
	awsE2EBSLConfig     = "region=us-west-2"
	awsE2EBSLBucketName = ""
	awsE2EBSLPrefix     = ""
	awsE2EVSLConfig     = "region=us-west-2"

	// TODO: fill in values
	azureE2EBSLConfig     = "resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,storageAccount=$AZURE_STORAGE_ACCOUNT_ID,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID"
	azureE2EBSLBucketName = ""
	azureE2EBSLPrefix     = ""
	azureE2EVSLConfig     = ""
)

// EnsureClusterExists returns whether or not a kubernetes cluster exists for tests to be run on.
func EnsureClusterExists(ctx context.Context) error {
	return exec.CommandContext(ctx, "kubectl", "cluster-info").Run()
}

// GetClusterClient instantiates and returns a client for the cluster.
func GetClusterClient() (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return client, nil
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

func NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	return false, nil
}
