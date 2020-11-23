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

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

var (
	backupName  string
	restoreName string
)

var _ = Describe("Backup Restore test using Kibishii to generate/verify data", func() {
	var client *kubernetes.Clientset
	BeforeEach(func() {
		flag.Parse()
		ctx := context.TODO()
		err := EnsureClusterExists(ctx)
		Expect(err).NotTo(HaveOccurred(), "Failed to ensure kubernetes cluster exists")
		client, err = GetClusterClient()
		Expect(err).NotTo(HaveOccurred(), "Failed to instantiate cluster client")
		println("Installing Velero")
		err = InstallVeleroServer(ctx, veleroCLI, veleroImage, cloudPlatform, cloudCredentialsFile)
		Expect(err).NotTo(HaveOccurred(), "Failed to install Velero in the cluster")
	})
	AfterEach(func() {
		println("Uninstalling Velero")
		timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
		err := client.CoreV1().Namespaces().Delete(timeoutCTX, "velero", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
	Describe("backing up and restoring namespace with data", func() {
		Context("when the backup is successful", func() {
			It("generates data, backups up the namespace, deletes the namespace, restores the namespace and verifies data", func() {
				backupUUID, err := uuid.NewRandom()
				Expect(err).NotTo(HaveOccurred())
				backupName = "backup-" + backupUUID.String()
				restoreName = "restore-" + backupUUID.String()
				println("backupName = " + backupName)
				println("creating namespace " + kibishiNamespace)
				timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
				err = CreateNamespace(timeoutCTX, client, kibishiNamespace)
				Expect(err).NotTo(HaveOccurred())

				println("installing kibishii in namespace " + kibishiNamespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = InstallKibishii(timeoutCTX, kibishiNamespace, cloudPlatform)
				Expect(err).NotTo(HaveOccurred())

				println("running kibishii generate")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*60)

				err = GenerateData(timeoutCTX, kibishiNamespace, 2, 10, 10, 1024, 1024, 0, 2)
				Expect(err).NotTo(HaveOccurred())

				println("executing backup")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*30)

				err = BackupNamespace(timeoutCTX, veleroCLI, backupName, kibishiNamespace)
				Expect(err).NotTo(HaveOccurred())
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = CheckBackupPhase(timeoutCTX, veleroCLI, backupName, velerov1.BackupPhaseCompleted)

				Expect(err).NotTo(HaveOccurred())

				println("removing namespace " + kibishiNamespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = client.CoreV1().Namespaces().Delete(timeoutCTX, kibishiNamespace, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				println("restoring namespace")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*30)
				err = RestoreNamespace(timeoutCTX, veleroCLI, restoreName, backupName)
				Expect(err).NotTo(HaveOccurred())
				println("Checking that namespace is present")
				// TODO - check that namespace exists
				println("running kibishii verify")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*60)

				err = VerifyData(timeoutCTX, kibishiNamespace, 2, 10, 10, 1024, 1024, 0, 2)
				Expect(err).NotTo(HaveOccurred())

				println("removing namespace " + kibishiNamespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = client.CoreV1().Namespaces().Delete(timeoutCTX, kibishiNamespace, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
