package e2e

import (
	"flag"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
)

var (
	uuidgen              uuid.UUID
	veleroInstallOptions *install.InstallOptions
)

func veleroInstall(pluginProvider string, useRestic bool) {
	var err error
	flag.Parse()
	Expect(EnsureClusterExists(context.TODO())).To(Succeed(), "Failed to ensure kubernetes cluster exists")
	uuidgen, err = uuid.NewRandom()
	Expect(err).To(Succeed())
	veleroInstallOptions, err = GetProviderVeleroInstallOptions(pluginProvider, cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, getProviderPlugins(pluginProvider))
	Expect(err).To(Succeed(), fmt.Sprintf("Failed to get Velero InstallOptions for plugin provider %s", pluginProvider))
	veleroInstallOptions.UseRestic = useRestic
	Expect(InstallVeleroServer(veleroInstallOptions)).To(Succeed(), "Failed to install Velero on KinD cluster")
}

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] [KinD] Velero tests on KinD cluster using the plugin provider for object storage and Restic for volume backups", func() {
	var (
		client      *kubernetes.Clientset
		backupName  string
		restoreName string
	)
	BeforeEach(func() {
		var err error
		veleroInstall(pluginProvider, true)
		client, err = GetClusterClient()
		Expect(err).To(Succeed(), "Failed to instantiate cluster client")
	})
	AfterEach(func() {
		timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
		err := client.CoreV1().Namespaces().Delete(timeoutCTX, veleroInstallOptions.Namespace, metav1.DeleteOptions{})
		Expect(err).To(Succeed())
	})
	Context("When kibishii is the sample workload", func() {
		It("should be successfully backed up and restored", func() {
			backupName = "backup-" + uuidgen.String()
			restoreName = "restore-" + uuidgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.
			Expect(RunKibishiiTests(client, "kind", veleroCLI, backupName, restoreName)).To(Succeed(), "Failed to successfully backup and restore Kibishii namespace")
		})
	})
})
