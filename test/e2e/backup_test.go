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
)

var (
	backupName  string
	restoreName string
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[AWS] Velero tests on AWS provider", func() {
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
	})
	AfterEach(func() {
		fmt.Printf("Uninstalling Velero")
		timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
		err := client.CoreV1().Namespaces().Delete(timeoutCTX, "velero", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
	Describe("Run Kibishii backup and restore test using AWS provider and Restic for volume backup", func() {
		Context("should install kibishii as test workload", func() {
			It("should successfully backup and restore kibishii workload", func() {

				io, err := GetProviderVeleroInstallOptions(providerName, cloudCredentialsFile)
				Expect(err).NotTo(HaveOccurred(), "Failed to get Velero InstallOptions for provider %s", providerName)
				io.UseRestic = true
				InstallVeleroServer(ctx, io)
				Expect(RunKibishiiTests(client, "aws", veleroCLI, backupName, restoreName)).NotTo(HaveOccurred(), "Failed to successfully backup and restore Kibishii namespace")
			})
		})
	})
})
