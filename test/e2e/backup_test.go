package e2e

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

var (
	uuidgen uuid.UUID
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", func() {
	var (
		client           *kubernetes.Clientset
		extensionsClient *apiextensionsclientset.Clientset
		backupName       string
		restoreName      string
	)

	BeforeEach(func() {
		var err error
		flag.Parse()
		uuidgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if installVelero {
			VeleroInstall(context.Background(), veleroImage, veleroNamespace, cloudProvider, objectStoreProvider, useVolumeSnapshots,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, "")
		}
		client, extensionsClient, err = kube.GetClusterClient()
		Expect(err).To(Succeed(), "Failed to instantiate cluster client")
	})

	AfterEach(func() {
		if installVelero {
			timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
			err := VeleroUninstall(timeoutCTX, client, extensionsClient, veleroNamespace)
			Expect(err).To(Succeed())
		}

	})

	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			backupName = "backup-" + uuidgen.String()
			restoreName = "restore-" + uuidgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.
			Expect(RunKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, "")).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})

		It("should successfully back up and restore to an additional BackupStorageLocation with unique credentials", func() {
			if additionalBSLProvider == "" {
				Skip("no additional BSL provider given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if additionalBSLBucket == "" {
				Skip("no additional BSL bucket given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			if additionalBSLCredentials == "" {
				Skip("no additional BSL credentials given, not running multiple BackupStorageLocation with unique credentials tests")
			}

			// Create Secret for additional BSL
			secretName := fmt.Sprintf("bsl-credentials-%s", uuidgen)
			secretKey := fmt.Sprintf("creds-%s", additionalBSLProvider)
			files := map[string]string{
				secretKey: additionalBSLCredentials,
			}

			Expect(CreateSecretFromFiles(context.TODO(), client, veleroNamespace, secretName, files)).To(Succeed())

			// Create additional BSL using credential
			additionalBsl := fmt.Sprintf("bsl-%s", uuidgen)
			Expect(VeleroCreateBackupLocation(context.TODO(),
				veleroCLI,
				veleroNamespace,
				additionalBsl,
				additionalBSLProvider,
				additionalBSLBucket,
				additionalBSLPrefix,
				additionalBSLConfig,
				secretName,
				secretKey,
			)).To(Succeed())

			bsls := []string{"default", additionalBsl}

			for _, bsl := range bsls {
				backupName = fmt.Sprintf("backup-%s-%s", bsl, uuidgen)
				restoreName = fmt.Sprintf("restore-%s-%s", bsl, uuidgen)

				Expect(RunKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, bsl)).To(Succeed(),
					"Failed to successfully backup and restore Kibishii namespace using BSL %s", bsl)
			}
		})
	})
})
