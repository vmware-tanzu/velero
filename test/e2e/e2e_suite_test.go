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
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
	. "github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/e2e/backup"
	. "github.com/vmware-tanzu/velero/test/e2e/backups"
	. "github.com/vmware-tanzu/velero/test/e2e/basic"
	. "github.com/vmware-tanzu/velero/test/e2e/basic/api-group"
	. "github.com/vmware-tanzu/velero/test/e2e/basic/backup-volume-info"
	. "github.com/vmware-tanzu/velero/test/e2e/basic/resources-check"
	. "github.com/vmware-tanzu/velero/test/e2e/bsl-mgmt"
	. "github.com/vmware-tanzu/velero/test/e2e/migration"
	. "github.com/vmware-tanzu/velero/test/e2e/privilegesmgmt"
	. "github.com/vmware-tanzu/velero/test/e2e/pv-backup"
	. "github.com/vmware-tanzu/velero/test/e2e/resource-filtering"
	. "github.com/vmware-tanzu/velero/test/e2e/resourcemodifiers"
	. "github.com/vmware-tanzu/velero/test/e2e/resourcepolicies"
	. "github.com/vmware-tanzu/velero/test/e2e/scale"
	. "github.com/vmware-tanzu/velero/test/e2e/schedule"
	. "github.com/vmware-tanzu/velero/test/e2e/upgrade"
	. "github.com/vmware-tanzu/velero/test/util/k8s"
	. "github.com/vmware-tanzu/velero/test/util/velero"
)

func init() {
	VeleroCfg.Options = install.Options{}
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
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroCLI, "upgrade-from-velero-cli", "", "comma-separated list of velero application for the pre-upgrade velero server.")
	flag.StringVar(&VeleroCfg.UpgradeFromVeleroVersion, "upgrade-from-velero-version", "v1.7.1", "comma-separated list of Velero version to be tested with for the pre-upgrade velero server.")
	flag.StringVar(&VeleroCfg.MigrateFromVeleroCLI, "migrate-from-velero-cli", "", "comma-separated list of velero application on source cluster.")
	flag.StringVar(&VeleroCfg.MigrateFromVeleroVersion, "migrate-from-velero-version", "self", "comma-separated list of Velero version to be tested with on source cluster.")
	flag.StringVar(&VeleroCfg.BSLConfig, "bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.BSLPrefix, "prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	flag.StringVar(&VeleroCfg.VSLConfig, "vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.VeleroNamespace, "velero-namespace", "velero", "namespace to install Velero into")
	flag.BoolVar(&InstallVelero, "install-velero", true, "install/uninstall velero during the test.  Optional.")
	flag.BoolVar(&VeleroCfg.UseNodeAgent, "use-node-agent", true, "whether deploy node agent daemonset velero during the test.  Optional.")
	flag.BoolVar(&VeleroCfg.UseVolumeSnapshots, "use-volume-snapshots", true, "whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider.")
	flag.StringVar(&VeleroCfg.RegistryCredentialFile, "registry-credential-file", "", "file containing credential for the image registry, follows the same format rules as the ~/.docker/config.json file. Optional.")
	flag.StringVar(&VeleroCfg.KibishiiDirectory, "kibishii-directory", "github.com/vmware-tanzu-experiments/distributed-data-generator/kubernetes/yaml/", "file directory or URL path to install Kibishii. Optional.")
	//vmware-tanzu-experiments
	// Flags to create an additional BSL for multiple credentials test
	flag.StringVar(&VeleroCfg.AdditionalBSLProvider, "additional-bsl-object-store-provider", "", "provider of object store plugin for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLBucket, "additional-bsl-bucket", "", "name of the object storage bucket for additional backup storage location. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.AdditionalBSLPrefix, "additional-bsl-prefix", "", "prefix under which all Velero data should be stored within the bucket for additional backup storage location. Optional.")
	flag.StringVar(&VeleroCfg.AdditionalBSLConfig, "additional-bsl-config", "", "configuration to use for the additional backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&VeleroCfg.AdditionalBSLCredentials, "additional-bsl-credentials-file", "", "file containing credentials for additional backup storage location provider. Required if testing multiple credentials support.")
	flag.StringVar(&VeleroCfg.Features, "features", "", "comma-separated list of features to enable for this Velero process.")
	flag.BoolVar(&VeleroCfg.Debug, "debug-e2e-test", false, "A Switch for enable or disable test data cleaning action.")
	flag.StringVar(&VeleroCfg.GCFrequency, "garbage-collection-frequency", "", "frequency of garbage collection.")
	flag.StringVar(&VeleroCfg.DefaultClusterContext, "default-cluster-context", "", "default cluster's kube config context, it's for migration test.")
	flag.StringVar(&VeleroCfg.StandbyClusterContext, "standby-cluster-context", "", "standby cluster's kube config context, it's for migration test.")
	flag.StringVar(&VeleroCfg.UploaderType, "uploader-type", "", "type of uploader for persistent volume backup.")
	flag.BoolVar(&VeleroCfg.VeleroServerDebugMode, "velero-server-debug-mode", false, "a switch for enable or disable having debug log of Velero server.")
	flag.BoolVar(&VeleroCfg.SnapshotMoveData, "snapshot-move-data", false, "a Switch for taking backup with Velero's data mover, if data-mover-plugin is not provided, using built-in plugin")
	flag.StringVar(&VeleroCfg.DataMoverPlugin, "data-mover-plugin", "", "customized plugin for data mover.")
	flag.StringVar(&VeleroCfg.StandbyClusterCloudProvider, "standby-cluster-cloud-provider", "", "cloud provider for standby cluster.")
	flag.StringVar(&VeleroCfg.StandbyClusterPlugins, "standby-cluster-plugins", "", "plugins provider for standby cluster.")
	flag.StringVar(&VeleroCfg.StandbyClusterObjectStoreProvider, "standby-cluster-object-store-provider", "", "object store provider for standby cluster.")
	flag.BoolVar(&VeleroCfg.DebugVeleroPodRestart, "debug-velero-pod-restart", false, "a switch for debugging velero pod restart.")
	flag.BoolVar(&VeleroCfg.DisableInformerCache, "disable-informer-cache", false, "a switch for disable informer cache.")
	flag.StringVar(&VeleroCfg.DefaultClusterName, "default-cluster-name", "", "default cluster's name in kube config file, it's for EKS IRSA test.")
	flag.StringVar(&VeleroCfg.StandbyClusterName, "standby-cluster-name", "", "standby cluster's name in kube config file, it's for EKS IRSA test.")
	flag.StringVar(&VeleroCfg.EKSPolicyARN, "eks-policy-arn", "", "EKS plicy ARN for creating AWS IAM service account.")
	flag.StringVar(&VeleroCfg.DefaultCLSServiceAccountName, "default-cls-service-account-name", "", "default cluster service account name.")
	flag.StringVar(&VeleroCfg.StandbyCLSServiceAccountName, "standby-cls-service-account-name", "", "standby cluster service account name.")
}

// Add label [SkipVanillaZfs]:
//   We found issue - https://github.com/openebs/zfs-localpv/issues/123 when using OpenEBS ZFS CSI Driver
//   When PVC using storage class with reclaim policy as 'Delete', once PVC is deleted, snapshot associated will be deleted
//   along with PVC deletion, after restoring workload, restored PVC is in pending status, due to failure of provision PV
//   caused by no expected snapshot found. If we use retain as reclaim policy, then this label can be ignored, all test
//   cases can be executed as expected successful result.

var _ = Describe("[APIGroup][APIVersion][SKIP_KIND] Velero tests with various CRD API group versions", APIGropuVersionsTest)
var _ = Describe("[APIGroup][APIExtensions][SKIP_KIND] CRD of apiextentions v1beta1 should be B/R successfully from cluster(k8s version < 1.22) to cluster(k8s version >= 1.22)", APIExtensionsVersionsTest)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Basic][Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupRestoreWithRestic)

var _ = Describe("[Basic][Snapshot][SkipVanillaZfs] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupRestoreWithSnapshots)

var _ = Describe("[Basic][Snapshot][RetainPV] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupRestoreRetainedPVWithSnapshots)

var _ = Describe("[Basic][Restic][RetainPV] Velero tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupRestoreRetainedPVWithRestic)

var _ = Describe("[Basic][ClusterResource] Backup/restore of cluster resources", ResourcesCheckTest)

var _ = Describe("[Scale][LongTime] Backup/restore of 2500 namespaces", MultiNSBackupRestore)

// Upgrade test by Kibishi using restic
var _ = Describe("[Upgrade][Restic] Velero upgrade tests on cluster using the plugin provider for object storage and Restic for volume backups", BackupUpgradeRestoreWithRestic)
var _ = Describe("[Upgrade][Snapshot][SkipVanillaZfs] Velero upgrade tests on cluster using the plugin provider for object storage and snapshots for volume backups", BackupUpgradeRestoreWithSnapshots)

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
var _ = Describe("[ResourceFiltering][ResourcePolicies][Restic] Velero test on skip backup of volume by resource policies", ResourcePoliciesTest)

// backup VolumeInfo test
var _ = Describe("[BackupVolumeInfo][SkippedVolume]", SkippedVolumeInfoTest)
var _ = Describe("[BackupVolumeInfo][FilesystemUpload]", FilesystemUploadVolumeInfoTest)
var _ = Describe("[BackupVolumeInfo][CSIDataMover]", CSIDataMoverVolumeInfoTest)
var _ = Describe("[BackupVolumeInfo][CSISnapshot]", CSISnapshotVolumeInfoTest)
var _ = Describe("[BackupVolumeInfo][NativeSnapshot]", NativeSnapshotVolumeInfoTest)

var _ = Describe("[ResourceModifier][Restore] Velero test on resource modifiers from the cluster restore", ResourceModifiersTest)

var _ = Describe("[Backups][Deletion][Restic] Velero tests of Restic backup deletion", BackupDeletionWithRestic)
var _ = Describe("[Backups][Deletion][Snapshot][SkipVanillaZfs] Velero tests of snapshot backup deletion", BackupDeletionWithSnapshots)
var _ = Describe("[Backups][TTL][LongTime][Snapshot][SkipVanillaZfs] Local backups and restic repos will be deleted once the corresponding backup storage location is deleted", TTLTest)
var _ = Describe("[Backups][BackupsSync] Backups in object storage are synced to a new Velero and deleted backups in object storage are synced to be deleted in Velero", BackupsSyncTest)

var _ = Describe("[Schedule][BR][Pause][LongTime] Backup will be created periodly by schedule defined by a Cron expression", ScheduleBackupTest)
var _ = Describe("[Schedule][OrderedResources] Backup resources should follow the specific order in schedule", ScheduleOrderedResources)
var _ = Describe("[Schedule][BackupCreation][SKIP_KIND] Schedule controller wouldn't create a new backup when it still has pending or InProgress backup", ScheduleBackupCreationTest)

var _ = Describe("[PrivilegesMgmt][SSR] Velero test on ssr object when controller namespace mix-ups", SSRTest)

var _ = Describe("[BSL][Deletion][Snapshot][SkipVanillaZfs] Local backups will be deleted once the corresponding backup storage location is deleted", BslDeletionWithSnapshots)
var _ = Describe("[BSL][Deletion][Restic] Local backups and restic repos will be deleted once the corresponding backup storage location is deleted", BslDeletionWithRestic)

var _ = Describe("[Migration][Restic] Migrate resources between clusters by Restic", MigrationWithRestic)
var _ = Describe("[Migration][Snapshot][SkipVanillaZfs] Migrate resources between clusters by snapshot", MigrationWithSnapshots)

var _ = Describe("[NamespaceMapping][Single][Restic] Backup resources should follow the specific order in schedule", OneNamespaceMappingResticTest)
var _ = Describe("[NamespaceMapping][Multiple][Restic] Backup resources should follow the specific order in schedule", MultiNamespacesMappingResticTest)
var _ = Describe("[NamespaceMapping][Single][Snapshot][SkipVanillaZfs] Backup resources should follow the specific order in schedule", OneNamespaceMappingSnapshotTest)
var _ = Describe("[NamespaceMapping][Multiple][Snapshot]SkipVanillaZfs] Backup resources should follow the specific order in schedule", MultiNamespacesMappingSnapshotTest)

var _ = Describe("[pv-backup][Opt-In] Backup resources should follow the specific order in schedule", OptInPVBackupTest)
var _ = Describe("[pv-backup][Opt-Out] Backup resources should follow the specific order in schedule", OptOutPVBackupTest)

var _ = Describe("[Basic][Nodeport] Service nodeport reservation during restore is configurable", NodePortTest)
var _ = Describe("[Basic][StorageClass] Storage class of persistent volumes and persistent volume claims can be changed during restores", StorageClasssChangingTest)
var _ = Describe("[Basic][SelectedNode][SKIP_KIND] Node selectors of persistent volume claims can be changed during restores", PVCSelectedNodeChangingTest)

func GetKubeconfigContext() error {
	var err error
	var tcDefault, tcStandby TestClient
	tcDefault, err = NewTestClient(VeleroCfg.DefaultClusterContext)
	VeleroCfg.DefaultClient = &tcDefault
	VeleroCfg.ClientToInstallVelero = VeleroCfg.DefaultClient
	VeleroCfg.ClusterToInstallVelero = VeleroCfg.DefaultClusterName
	VeleroCfg.ServiceAccountNameToInstall = VeleroCfg.DefaultCLSServiceAccountName
	if err != nil {
		return err
	}

	if VeleroCfg.DefaultClusterContext != "" {
		err = KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultClusterContext)
		if err != nil {
			return err
		}
		if VeleroCfg.StandbyClusterContext != "" {
			tcStandby, err = NewTestClient(VeleroCfg.StandbyClusterContext)
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
	// 2. E2E tests require a Kubernetes cluster to install and run velero which further requires more configuration. See above referenced command line flags.
	if testing.Short() {
		t.Skip("Skipping E2E tests")
	}

	if !slices.Contains(LocalCloudProviders, VeleroCfg.CloudProvider) {
		fmt.Println("For cloud platforms, object store plugin provider will be set as cloud provider")
		// If ObjectStoreProvider is not provided, then using the value same as CloudProvider
		if VeleroCfg.ObjectStoreProvider == "" {
			VeleroCfg.ObjectStoreProvider = VeleroCfg.CloudProvider
		}
	} else {
		if VeleroCfg.ObjectStoreProvider == "" {
			t.Error(errors.New("No object store provider specified - must be specified when using kind as the cloud provider")) // Must have an object store provider
		}
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

var _ = BeforeSuite(func() {
	if InstallVelero {
		By("Install test resources before testing")
		Expect(PrepareVelero(context.Background(), "install resource before testing", VeleroCfg)).To(Succeed())
	}
})

var _ = AfterSuite(func() {
	if InstallVelero && !VeleroCfg.Debug {
		By("release test resources after testing")
		ctx, ctxCancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer ctxCancel()
		Expect(VeleroUninstall(ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace)).To(Succeed())
	}
})
