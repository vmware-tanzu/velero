package e2e

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	uuidgen uuid.UUID
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", backup_restore_with_restic)

var _ = Describe("[Snapshot] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", backup_restore_with_snapshots)

func backup_restore_with_snapshots() {
	backup_restore_test(true)
}

func backup_restore_with_restic() {
	backup_restore_test(false)
}

func backup_restore_test(useVolumeSnapshots bool) {
	var (
		backupName  string
		restoreName string
	)

	client, err := NewTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for backup tests")

	BeforeEach(func() {
		if useVolumeSnapshots && cloudProvider == "kind" {
			Skip("Volume snapshots not supported on kind")
		}
		var err error
		flag.Parse()
		uuidgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if installVelero {
			Expect(VeleroInstall(context.Background(), veleroImage, veleroNamespace, cloudProvider, objectStoreProvider, useVolumeSnapshots,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, "")).To(Succeed())

		}
	})

	AfterEach(func() {
		err = VeleroUninstall(client.Kubebuilder, installVelero, veleroNamespace)
		Expect(err).To(Succeed())
	})

	When("kibishii is the sample workload", func() {
		It("should be successfully backed up and restored to the default BackupStorageLocation", func() {
			backupName = "backup-" + uuidgen.String()
			restoreName = "restore-" + uuidgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.
			Expect(RunKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, "", useVolumeSnapshots)).To(Succeed(),
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

			Expect(VeleroAddPluginsForProvider(context.TODO(), veleroCLI, veleroNamespace, additionalBSLProvider)).To(Succeed())

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

				Expect(RunKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName, bsl, useVolumeSnapshots)).To(Succeed(),
					"Failed to successfully backup and restore Kibishii namespace using BSL %s", bsl)
			}
		})
	})
}
