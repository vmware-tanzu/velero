/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/backup"
	. "github.com/vmware-tanzu/velero/test/e2e/backups"
	. "github.com/vmware-tanzu/velero/test/e2e/basic"
	. "github.com/vmware-tanzu/velero/test/e2e/basic/resources-check"
	. "github.com/vmware-tanzu/velero/test/e2e/bsl-mgmt"
	. "github.com/vmware-tanzu/velero/test/e2e/migration"
	. "github.com/vmware-tanzu/velero/test/e2e/privilegesmgmt"
	. "github.com/vmware-tanzu/velero/test/e2e/pv-backup"
	. "github.com/vmware-tanzu/velero/test/e2e/resource-filtering"
	. "github.com/vmware-tanzu/velero/test/e2e/scale"
	. "github.com/vmware-tanzu/velero/test/e2e/schedule"
	. "github.com/vmware-tanzu/velero/test/e2e/upgrade"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

func init() {
	flag.StringVar(&VeleroCfg.CloudProvider, "cloud-provider", "", "cloud that Velero will be installed into.  Required.")
	flag.StringVar(&VeleroCfg.ObjectStoreProvider, "object-store-provider", "", "provider of object store plugin. Required if cloud-provider is kind, otherwise ignored.")
	flag.StringVar(&VeleroCfg.BSLBucket, "bucket", "", "name of the object storage bucket where backups from e2e tests should be stored. Required.")
	flag.StringVar(&VeleroCfg.CloudCredentialsFile, "credentials-file", "", "file containing credentials for backup and volume provider. Required.")
	flag.StringVar(&VeleroCfg.VeleroCLI, "velerocli", "velero", "path to the velero application to use.")
	flag.StringVar(&VeleroCfg.VeleroImage, "velero-image", "velero/velero:main", "image for the velero server to be tested.")
	flag.StringVar(&VeleroCfg.Plugins, "plugins", "", "provider plugins to be tested.")
	flag.StringVar(&VeleroCfg.AddBSLPlugins, "additional-bsl-plugins", "", "additional plugins to be tested.")
	flag.StringVar(&VeleroCfg.VeleroVersion, "velero-version", "main", "image version for the velero server to be tested with.")
	flag.StringVar(&VeleroCfg.RestoreHelperImage, "restore-helper-image", "", "image for the velero restore helper to be tested.")
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroCLI, "upgrade-from-velero-cli", "", "path to the pre-upgrade velero application to use.")
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroVersion, "upgrade-from-velero-version", "v1.7.1", "image for the pre-upgrade velero server to be tested.")
	flag.StringVar(&VeleroCfg.MigrateFromVeleroCLI, "migrate-from-velero-cli", "", "path to the origin velero application to use.")
	flag.StringVar(&VeleroCfg.MigrateFromVeleroVersion, "migrate-from-velero-version", "self", "image for the origin velero server to be tested.")
	flag.StringVar(&VeleroCfg.BSLConfig, "bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.BSLPrefix, "prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	flag.StringVar(&VeleroCfg.VSLConfig, "vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.VeleroNamespace, "velero-namespace", "velero", "namespace to install Velero into")
	flag.BoolVar(&VeleroCfg.InstallVelero, "install-velero", true, "install/uninstall velero during the test.  Optional.")
	flag.StringVar(&VeleroCfg.RegistryCredentialFile, "registry-credential-file", "", "file containing credential for the image registry, follows the same format rules as the ~/.docker/config.json file. Optional.")
	flag.StringVar(&VeleroCfg.KibishiiDirectory, "kibishii-directory", "github.com/vmware-tanzu-experiments/distributed-data-generator/kubernetes/yaml/", "The file directory or URL path to install Kibishii. Optional.")
	//vmware-tanzu-experiments
	// Flags to create an additional BSL for multiple credentials test
	flag.StringVar(&VeleroCfg.AdditionalBSLProvider, "additional-bsl-object-store-provider", "", "Provider of object store plugin for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLBucket, "additional-bsl-bucket", "", "name of the object storage bucket for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLPrefix, "additional-bsl-prefix", "", "prefix under which all Velero data should be stored within the bucket for additional backup storage location. Optional.")
	flag.StringVar(&VeleroCfg.AdditionalBSLConfig, "additional-bsl-config", "", "configuration to use for the additional backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.AdditionalBSLCredentials, "additional-bsl-credentials-file", "", "file containing credentials for additional backup storage location provider. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.Features, "features", "", "Comma-separated list of features to enable for this Velero process.")
	flag.BoolVar(&VeleroCfg.Debug, "debug-e2e-test", false, "Switch to control namespace cleaning.")
	flag.StringVar(&VeleroCfg.GCFrequency, "garbage-collection-frequency", "", "Frequency of garbage collection.")
	flag.StringVar(&VeleroCfg.DefaultCluster, "default-cluster", "", "Default cluster context for migration test.")
	flag.StringVar(&VeleroCfg.StandbyCluster, "standby-cluster", "", "Standby cluster context for migration test.")
	flag.StringVar(&VeleroCfg.UploaderType, "uploader-type", "", "Identify persistent volume backup uploader.")
	flag.BoolVar(&VeleroCfg.VeleroServerDebugMode, "velero-server-debug-mode", false, "Identify persistent volume backup uploader.")

}

var _ = Describe("[APIGroup][Common] Velero tests with various CRD API group versions", APIGropuVersionsTest)
var _ = Describe("[APIGroup][APIExtensions] CRD of apiextentions v1beta1 should be B/R successfully from cluster(k8s version < 1.22) to cluster(k8s version >= 1.22)", APIExtensionsVersionsTest)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Basic][Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupRestoreWithRestic)

var _ = Describe("[Basic][Snapshot] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupRestoreWithSnapshots)

var _ = Describe("[Basic][ClusterResource] Backup/restore of cluster resources", ResourcesCheckTest)

var _ = Describe("[Scale][LongTime] Backup/restore of 2500 namespaces", MultiNSBackupRestore)

// Upgrade test by Kibishi using restic
var _ = Describe("[Upgrade][Restic] Velero upgrade tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupUpgradeRestoreWithRestic)
var _ = Describe("[Upgrade][Snapshot] Velero upgrade tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupUpgradeRestoreWithSnapshots)

// test filter objects by namespace, type, or labels when backup or restore.
var _ = Describe("[ResourceFiltering][ExcludeFromBackup] Resources with the label velero.io/exclude-from-backup=true are not included in backup", ExcludeFromBackupTest)
var _ = Describe("[ResourceFiltering][ExcludeNamespaces][Backup] Velero test on exclude namespace from the cluster backup", BackupWithExcludeNamespaces)
var _ = Describe("[ResourceFiltering][ExcludeNamespaces][Restore] Velero test on exclude namespace from the cluster restore", RestoreWithExcludeNamespaces)
var _ = Describe("[ResourceFiltering][ExcludeResources][Backup] Velero test on exclude resources from the cluster backup", BackupWithExcludeResources)
var _ = Describe("[ResourceFiltering][ExcludeResources][Restore] Velero test on exclude resources from the cluster restore", RestoreWithExcludeResources)
var _ = Describe("[ResourceFiltering][IncludeNamespaces][Backup] Velero test on include namespace from the cluster backup", BackupWithIncludeNamespaces)
var _ = Describe("[ResourceFiltering][IncludeNamespaces][Restore] Velero test on include namespace from the cluster restore", RestoreWithIncludeNamespaces)
var _ = Describe("[ResourceFiltering][IncludeResources][Backup] Velero test on include resources from the cluster backup", BackupWithIncludeResources)
var _ = Describe("[ResourceFiltering][IncludeResources][Restore] Velero test on include resources from the cluster restore", RestoreWithIncludeResources)
var _ = Describe("[ResourceFiltering][LabelSelector] Velero test on backup include resources matching the label selector", BackupWithLabelSelector)

var _ = Describe("[Backups][Deletion][Restic] Velero tests of Restic backup deletion", BackupDeletionWithRestic)
var _ = Describe("[Backups][Deletion][Snapshot] Velero tests of snapshot backup deletion", BackupDeletionWithSnapshots)
var _ = Describe("[Backups][TTL][LongTime] Local backups and restic repos will be deleted once the corresponding backup storage location is deleted", TTLTest)
var _ = Describe("[Backups][BackupsSync] Backups in object storage are synced to a new Velero and deleted backups in object storage are synced to be deleted in Velero", BackupsSyncTest)

var _ = Describe("[Schedule][BR][Pause][LongTime] Backup will be created periodly by schedule defined by a Cron expression", ScheduleBackupTest)
var _ = Describe("[Schedule][OrederedResources] Backup resources should follow the specific order in schedule", ScheduleOrderedResources)
var _ = Describe("[Schedule][BackupCreation] Schedule controller wouldn't create a new backup when it still has pending or InProgress backup", ScheduleBackupCreationTest)

var _ = Describe("[PrivilegesMgmt][SSR] Velero test on ssr object when controller namespace mix-ups", SSRTest)

var _ = Describe("[BSL][Deletion][Snapshot] Local backups will be deleted once the corresponding backup storage location is deleted", BslDeletionWithSnapshots)
var _ = Describe("[BSL][Deletion][Restic] Local backups and restic repos will be deleted once the corresponding backup storage location is deleted", BslDeletionWithRestic)

var _ = Describe("[Migration][Restic] Migrate resources between clusters by Restic", MigrationWithRestic)
var _ = Describe("[Migration][Snapshot] Migrate resources between clusters by snapshot", MigrationWithSnapshots)

var _ = Describe("[NamespaceMapping][Single][Restic] Backup resources should follow the specific order in schedule", OneNamespaceMappingResticTest)
var _ = Describe("[NamespaceMapping][Multiple][Restic] Backup resources should follow the specific order in schedule", MultiNamespacesMappingResticTest)
var _ = Describe("[NamespaceMapping][Single][Snapshot] Backup resources should follow the specific order in schedule", OneNamespaceMappingSnapshotTest)
var _ = Describe("[NamespaceMapping][Multiple][Snapshot] Backup resources should follow the specific order in schedule", MultiNamespacesMappingSnapshotTest)

var _ = Describe("[pv-backup][Opt-In] Backup resources should follow the specific order in schedule", OptInPVBackupTest)
var _ = Describe("[pv-backup][Opt-Out] Backup resources should follow the specific order in schedule", OptOutPVBackupTest)

var _ = Describe("[Basic][Nodeport] Service nodeport reservation during restore is configurable", NodePortTest)
var _ = Describe("[Basic][StorageClass] Storage class of persistent volumes and persistent volume claims can be changed during restores", StorageClasssChangingTest)
var _ = Describe("[Basic][SelectedNode] Node selectors of persistent volume claims can be changed during restores", PVCSelectedNodeChangingTest)

func GetKubeconfigContext() error {
	var err error
	var tcDefault, tcStandby TestClient
	tcDefault, err = NewTestClient(VeleroCfg.DefaultCluster)
	VeleroCfg.DefaultClient = &tcDefault
	VeleroCfg.ClientToInstallVelero = VeleroCfg.DefaultClient
	if err != nil {
		return err
	}

	if VeleroCfg.DefaultCluster != "" {
		err = KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultCluster)
		if err != nil {
			return err
		}
		if VeleroCfg.StandbyCluster != "" {
			tcStandby, err = NewTestClient(VeleroCfg.StandbyCluster)
			VeleroCfg.StandbyClient = &tcStandby
			if err != nil {
				return err
			}
		} else {
			return errors.New("migration test needs 2 clusters to run")
		}
	}

	return nil
}

func TestE2e(t *testing.T) {
	// Skip running E2E tests when running only "short" tests because:
	// 1. E2E tests are long running tests involving installation of Velero and performing backup and restore operations.
	// 2. E2E tests require a kubernetes cluster to install and run velero which further requires more configuration. See above referenced command line flags.
	if testing.Short() {
		t.Skip("Skipping E2E tests")
	}

	var err error
	if err = GetKubeconfigContext(); err != nil {
		fmt.Println(err)
		t.FailNow()
	}

	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("report.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "E2e Suite", []Reporter{junitReporter})
}
