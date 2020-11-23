package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/install"
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

func getCloudPlatformPlugins(cloudPlatform string) []string {
	// TODO: make plugin images configurable
	switch cloudPlatform {
	case "kind":
		return []string{"velero/velero-plugin-for-aws:v1.1.0"}
	case "azure":
		return []string{"velero/velero-plugin-for-microsoft-azure:v1.1.1"}
	case "vsphere":
		// TODO: find correct image tag
		return []string{"velero/velero-plugin-for-vsphere:v1.1.1"}
	default:
		return []string{""}
	}
}

func getVeleroInstallOptions(cloudPlatform, credentialsFile string) (install.VeleroOptions, error) {
	// TODO: make this configurable
	realPath, err := filepath.Abs(credentialsFile)
	if err != nil {
		return install.VeleroOptions{}, err
	}

	secretData, err := ioutil.ReadFile(realPath)
	if err != nil {
		return install.VeleroOptions{}, err
	}
	return install.VeleroOptions{
		Namespace:    "velero",
		Image:        "velero/velero:main",
		ProviderName: "aws",
		Bucket:       "ashisha-velero-backup",
		Prefix:       "velero-backup",
		// PodAnnotations                    map[string]string
		// ServiceAccountAnnotations         map[string]string
		// VeleroPodResources                corev1.ResourceRequirements
		// ResticPodResources                corev1.ResourceRequirements
		SecretData: secretData,
		// RestoreOnly                       bool
		UseRestic: true,
		// UseVolumeSnapshots                bool
		BSLConfig: map[string]string{"region": "us-west-2"},
		VSLConfig: map[string]string{"region": "us-west-2"},
		// DefaultResticMaintenanceFrequency time.Duration
		Plugins: getCloudPlatformPlugins(cloudPlatform),
		// NoDefaultBackupLocation           bool
		// CACertData                        []byte
		// Features                          []string
		// DefaultVolumesToRestic            bool

	}, nil
}

// InstallVeleroServer installs velero in the cluster using `velero install command`
func InstallVeleroServer(ctx context.Context, veleroCli, veleroImage, cloudPlatform, cloudCredentialsFile string) error {
	config, err := client.LoadConfig()
	if err != nil {
		return err
	}
	f := client.NewFactory("e2e", config)
	vo, err := getVeleroInstallOptions(cloudPlatform, cloudCredentialsFile)
	if err != nil {
		return err
	}
	resources, err := install.AllResources(&vo)
	if err != nil {
		return err
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	factory := client.NewDynamicFactory(dynamicClient)
	errorMsg := "\n\nError installing Velero. Use `kubectl logs deploy/velero -n velero` to check the deploy logs"
	err = install.Install(factory, resources, os.Stdout)
	if err != nil {
		return errors.Wrap(err, errorMsg)
	}

	fmt.Println("Waiting for Velero deployment to be ready.")
	if _, err = install.DeploymentIsReady(factory, "velero"); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	// restic enabled by default
	fmt.Println("Waiting for Velero restic daemonset to be ready.")
	if _, err = install.DaemonSetIsReady(factory, "velero"); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	return nil
}
