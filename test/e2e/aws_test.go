package e2e

import (
	"flag"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	backupName  string
	restoreName string
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[AWS] [Kind] Velero tests on Kind cluster using AWS provider for object storage", func() {
	var (
		client      *kubernetes.Clientset
		uuidgen     uuid.UUID
		ctx         context.Context
		err         error
		backupName  string
		restoreName string
	)
	BeforeEach(func() {
		flag.Parse()
		ctx = context.TODO()
		Expect(EnsureClusterExists(ctx)).NotTo(HaveOccurred(), "Failed to ensure kubernetes cluster exists")
		client, err = GetClusterClient()
		Expect(err).NotTo(HaveOccurred(), "Failed to instantiate cluster client")
		uuidgen, err = uuid.NewRandom()
		Expect(err).NotTo(HaveOccurred())
		backupName = "backup-" + uuidgen.String()
		restoreName = "restore-" + uuidgen.String()
		io, err := GetProviderVeleroInstallOptions("aws", cloudCredentialsFile)
		Expect(err).NotTo(HaveOccurred(), "Failed to get Velero InstallOptions for provider %aws")
		io.UseRestic = true
		Expect(InstallVeleroServer(ctx, io)).NotTo(HaveOccurred(), "Failed to install Velero on kind cluster")
	})
	AfterEach(func() {
		timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
		err := client.CoreV1().Namespaces().Delete(timeoutCTX, "velero", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
	Describe("Run Kibishii backup and restore test using AWS provider and Restic for volume backup", func() {
		Context("should install kibishii as test workload", func() {
			It("should successfully backup and restore kibishii workload", func() {
				// Even though we are using Velero's AWS plugin for object storage, the kubernetes cluster is running on
				// KinD. So use the kind installation for Kibishii.
				Expect(RunKibishiiTests(client, "kind", veleroCLI, backupName, restoreName)).NotTo(HaveOccurred(), "Failed to successfully backup and restore Kibishii namespace")
			})
		})
	})
})
