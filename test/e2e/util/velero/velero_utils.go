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

package velero

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	ver "k8s.io/apimachinery/pkg/util/version"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	cliinstall "github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	. "github.com/vmware-tanzu/velero/test/e2e"
	common "github.com/vmware-tanzu/velero/test/e2e/util/common"
	util "github.com/vmware-tanzu/velero/test/e2e/util/csi"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
)

const BackupObjectsPrefix = "backups"
const RestoreObjectsPrefix = "restores"
const PluginsObjectsPrefix = "plugins"

var pluginsMatrix = map[string]map[string][]string{
	"v1.4": {
		"aws":       {"velero/velero-plugin-for-aws:v1.1.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.1.2"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.1.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.0.2"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.1.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.1.2", "velero/velero-plugin-for-csi:v0.1.1 "},
	},
	"v1.5": {
		"aws":       {"velero/velero-plugin-for-aws:v1.1.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.1.2"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.1.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.1.1"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.1.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.1.2", "velero/velero-plugin-for-csi:v0.1.2 "},
	},
	"v1.6": {
		"aws":       {"velero/velero-plugin-for-aws:v1.2.1"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.2.1"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.2.1", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.1.1"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.2.1"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.3.0", "velero/velero-plugin-for-csi:v0.1.2 "},
	},
	"v1.7": {
		"aws":       {"velero/velero-plugin-for-aws:v1.3.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.3.0"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.3.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.3.0"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.3.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.3.0", "velero/velero-plugin-for-csi:v0.2.0"},
	},
	"v1.8": {
		"aws":       {"velero/velero-plugin-for-aws:v1.4.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.4.0"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.4.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.3.1"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.4.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.4.0", "velero/velero-plugin-for-csi:v0.2.0"},
	},
	"v1.9": {
		"aws":       {"velero/velero-plugin-for-aws:v1.5.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.5.0"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.5.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.4.0"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.5.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.5.0", "velero/velero-plugin-for-csi:v0.3.0"},
	},
	"v1.10": {
		"aws":       {"velero/velero-plugin-for-aws:v1.6.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.6.0"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.6.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.4.1"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.6.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.6.0", "velero/velero-plugin-for-csi:v0.4.0"},
	},
	"v1.11": {
		"aws":       {"velero/velero-plugin-for-aws:v1.7.0"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:v1.7.0"},
		"vsphere":   {"velero/velero-plugin-for-aws:v1.7.0", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.4.2"},
		"gcp":       {"velero/velero-plugin-for-gcp:v1.7.0"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:v1.7.0", "velero/velero-plugin-for-csi:v0.5.0"},
	},
	"main": {
		"aws":       {"velero/velero-plugin-for-aws:main"},
		"azure":     {"velero/velero-plugin-for-microsoft-azure:main"},
		"vsphere":   {"velero/velero-plugin-for-aws:main", "vsphereveleroplugin/velero-plugin-for-vsphere:v1.3.1"},
		"gcp":       {"velero/velero-plugin-for-gcp:main"},
		"azure-csi": {"velero/velero-plugin-for-microsoft-azure:main", "velero/velero-plugin-for-csi:main"},
	},
}

func GetProviderPluginsByVersion(version, providerName, feature string) ([]string, error) {
	var cloudMap map[string][]string
	arr := strings.Split(version, ".")
	if len(arr) >= 3 {
		cloudMap = pluginsMatrix[arr[0]+"."+arr[1]]
	}
	if len(cloudMap) == 0 {
		cloudMap = pluginsMatrix["main"]
		if len(cloudMap) == 0 {
			return nil, errors.Errorf("fail to get plugins by version: main")
		}
	}
	if strings.EqualFold(providerName, "azure") && strings.EqualFold(feature, "EnableCSI") {
		providerName = "azure-csi"
	}
	plugins, ok := cloudMap[providerName]
	if !ok {
		return nil, errors.Errorf("fail to get plugins by version: %s and provider %s", version, providerName)
	}
	return plugins, nil
}

// getProviderVeleroInstallOptions returns Velero InstallOptions for the provider.
func getProviderVeleroInstallOptions(veleroCfg *VeleroConfig,
	plugins []string) (*cliinstall.InstallOptions, error) {

	if veleroCfg.CloudCredentialsFile == "" {
		return nil, errors.Errorf("No credentials were supplied to use for E2E tests")
	}

	realPath, err := filepath.Abs(veleroCfg.CloudCredentialsFile)
	if err != nil {
		return nil, err
	}

	io := cliinstall.NewInstallOptions()
	// always wait for velero and restic pods to be running.
	io.Wait = true
	io.ProviderName = veleroCfg.ObjectStoreProvider
	io.SecretFile = veleroCfg.CloudCredentialsFile

	io.BucketName = veleroCfg.BSLBucket
	io.Prefix = veleroCfg.BSLPrefix
	io.BackupStorageConfig = flag.NewMap()
	io.BackupStorageConfig.Set(veleroCfg.BSLConfig)

	io.VolumeSnapshotConfig = flag.NewMap()
	io.VolumeSnapshotConfig.Set(veleroCfg.VSLConfig)

	io.SecretFile = realPath
	io.Plugins = flag.NewStringArray(plugins...)
	io.Features = veleroCfg.Features
	io.DefaultVolumesToFsBackup = veleroCfg.DefaultVolumesToFsBackup
	io.UseVolumeSnapshots = veleroCfg.UseVolumeSnapshots
	return io, nil
}

// checkBackupPhase uses VeleroCLI to inspect the phase of a Velero backup.
func checkBackupPhase(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string,
	expectedPhase velerov1api.BackupPhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "backup", "get", "-o", "json",
		backupName)

	fmt.Printf("get backup cmd =%v\n", checkCMD)
	jsonBuf, err := common.CMDExecWithOutput(checkCMD)
	if err != nil {
		return err
	}
	backup := velerov1api.Backup{}
	err = json.Unmarshal(*jsonBuf, &backup)
	if err != nil {
		return err
	}
	if backup.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected backup phase got %s, expecting %s", backup.Status.Phase, expectedPhase)
	}
	return nil
}

// checkRestorePhase uses VeleroCLI to inspect the phase of a Velero restore.
func checkRestorePhase(ctx context.Context, veleroCLI string, veleroNamespace string, restoreName string,
	expectedPhase velerov1api.RestorePhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "restore", "get", "-o", "json",
		restoreName)

	fmt.Printf("get restore cmd =%v\n", checkCMD)
	jsonBuf, err := common.CMDExecWithOutput(checkCMD)
	if err != nil {
		return err
	}
	restore := velerov1api.Restore{}
	err = json.Unmarshal(*jsonBuf, &restore)
	if err != nil {
		return err
	}
	if restore.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected restore phase got %s, expecting %s", restore.Status.Phase, expectedPhase)
	}
	return nil
}

func checkSchedulePhase(ctx context.Context, veleroCLI, veleroNamespace, scheduleName string) error {
	return wait.PollImmediate(time.Second*5, time.Minute*2, func() (bool, error) {
		checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "schedule", "get", scheduleName, "-ojson")
		jsonBuf, err := common.CMDExecWithOutput(checkCMD)
		if err != nil {
			return false, err
		}
		schedule := velerov1api.Schedule{}
		err = json.Unmarshal(*jsonBuf, &schedule)
		if err != nil {
			return false, err
		}

		if schedule.Status.Phase != velerov1api.SchedulePhaseEnabled {
			fmt.Printf("Unexpected schedule phase got %s, expecting %s, still waiting...", schedule.Status.Phase, velerov1api.SchedulePhaseEnabled)
			return false, nil
		}
		return true, nil
	})
}

func checkSchedulePause(ctx context.Context, veleroCLI, veleroNamespace, scheduleName string, pause bool) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "schedule", "get", scheduleName, "-ojson")
	jsonBuf, err := common.CMDExecWithOutput(checkCMD)
	if err != nil {
		return err
	}
	schedule := velerov1api.Schedule{}
	err = json.Unmarshal(*jsonBuf, &schedule)
	if err != nil {
		return err
	}

	if schedule.Spec.Paused != pause {
		fmt.Printf("Unexpected schedule phase got %s, expecting %s, still waiting...", schedule.Status.Phase, velerov1api.SchedulePhaseEnabled)
		return nil
	}
	return nil
}
func CheckScheduleWithResourceOrder(ctx context.Context, veleroCLI, veleroNamespace, scheduleName string, order map[string]string) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "schedule", "get", scheduleName, "-ojson")
	jsonBuf, err := common.CMDExecWithOutput(checkCMD)
	if err != nil {
		return err
	}
	schedule := velerov1api.Schedule{}
	err = json.Unmarshal(*jsonBuf, &schedule)
	if err != nil {
		return err
	}

	if schedule.Status.Phase != velerov1api.SchedulePhaseEnabled {
		return errors.Errorf("Unexpected restore phase got %s, expecting %s", schedule.Status.Phase, velerov1api.SchedulePhaseEnabled)
	}
	if reflect.DeepEqual(schedule.Spec.Template.OrderedResources, order) {
		return nil
	} else {
		return fmt.Errorf("resource order %v set in schedule command is not equal with order %v stored in schedule cr", order, schedule.Spec.Template.OrderedResources)
	}
}

func CheckBackupWithResourceOrder(ctx context.Context, veleroCLI, veleroNamespace, backupName string, order map[string]string) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "get", "backup", backupName, "-ojson")
	jsonBuf, err := common.CMDExecWithOutput(checkCMD)
	if err != nil {
		return err
	}
	backup := velerov1api.Backup{}
	err = json.Unmarshal(*jsonBuf, &backup)
	if err != nil {
		return err
	}
	if backup.Status.Phase != velerov1api.BackupPhaseCompleted {
		return errors.Errorf("Unexpected restore phase got %s, expecting %s", backup.Status.Phase, velerov1api.BackupPhaseCompleted)
	}
	if reflect.DeepEqual(backup.Spec.OrderedResources, order) {
		return nil
	} else {
		return fmt.Errorf("resource order %v set in backup command is not equal with order %v stored in backup cr", order, backup.Spec.OrderedResources)
	}
}

// VeleroBackupNamespace uses the veleroCLI to backup a namespace.
func VeleroBackupNamespace(ctx context.Context, veleroCLI, veleroNamespace string, backupCfg BackupConfig) error {
	args := []string{
		"--namespace", veleroNamespace,
		"create", "backup", backupCfg.BackupName,
		"--wait",
	}
	if backupCfg.Namespace != "" {
		args = append(args, "--include-namespaces", backupCfg.Namespace)
	}
	if backupCfg.Selector != "" {
		args = append(args, "--selector", backupCfg.Selector)
	}

	if backupCfg.UseVolumeSnapshots {
		if backupCfg.ProvideSnapshotsVolumeParam {
			args = append(args, "--snapshot-volumes")
		}
	}
	if backupCfg.DefaultVolumesToFsBackup {
		if backupCfg.UseResticIfFSBackup {
			args = append(args, "--default-volumes-to-restic")
		} else {
			args = append(args, "--default-volumes-to-fs-backup")
		}

		// To workaround https://github.com/vmware-tanzu/velero-plugin-for-vsphere/issues/347 for vsphere plugin v1.1.1
		// if the "--snapshot-volumes=false" isn't specified explicitly, the vSphere plugin will always take snapshots
		// for the volumes even though the "--default-volumes-to-fs-backup" is specified
		// TODO This can be removed if the logic of vSphere plugin bump up to 1.3
		if backupCfg.ProvideSnapshotsVolumeParam && !backupCfg.UseVolumeSnapshots {
			args = append(args, "--snapshot-volumes=false")
		} // if "--snapshot-volumes" is not provide, snapshot should be taken as default behavior.
	}
	if backupCfg.BackupLocation != "" {
		args = append(args, "--storage-location", backupCfg.BackupLocation)
	}
	if backupCfg.TTL != 0 {
		args = append(args, "--ttl", backupCfg.TTL.String())
	}

	if backupCfg.IncludeResources != "" {
		args = append(args, "--include-resources", backupCfg.IncludeResources)
	}

	if backupCfg.ExcludeResources != "" {
		args = append(args, "--exclude-resources", backupCfg.ExcludeResources)
	}

	if backupCfg.IncludeClusterResources {
		args = append(args, "--include-cluster-resources")
	}

	if backupCfg.OrderedResources != "" {
		args = append(args, "--ordered-resources", backupCfg.OrderedResources)
	}

	return VeleroBackupExec(ctx, veleroCLI, veleroNamespace, backupCfg.BackupName, args)
}

// VeleroBackupExcludeNamespaces uses the veleroCLI to backup a namespace.
func VeleroBackupExcludeNamespaces(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string, excludeNamespaces []string) error {
	namespaces := strings.Join(excludeNamespaces, ",")
	args := []string{
		"--namespace", veleroNamespace, "create", "backup", backupName,
		"--exclude-namespaces", namespaces,
		"--default-volumes-to-fs-backup", "--wait",
	}
	return VeleroBackupExec(ctx, veleroCLI, veleroNamespace, backupName, args)
}

// VeleroBackupIncludeNamespaces uses the veleroCLI to backup a namespace.
func VeleroBackupIncludeNamespaces(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string, includeNamespaces []string) error {
	namespaces := strings.Join(includeNamespaces, ",")
	args := []string{
		"--namespace", veleroNamespace, "create", "backup", backupName,
		"--include-namespaces", namespaces,
		"--default-volumes-to-fs-backup", "--wait",
	}
	return VeleroBackupExec(ctx, veleroCLI, veleroNamespace, backupName, args)
}

// VeleroRestore uses the VeleroCLI to restore from a Velero backup.
func VeleroRestore(ctx context.Context, veleroCLI, veleroNamespace, restoreName, backupName, includeResources string) error {
	args := []string{
		"--namespace", veleroNamespace, "create", "restore", restoreName,
		"--from-backup", backupName, "--wait",
	}
	if includeResources != "" {
		args = append(args, "--include-resources", includeResources)
	}
	return VeleroRestoreExec(ctx, veleroCLI, veleroNamespace, restoreName, args, velerov1api.RestorePhaseCompleted)
}

func VeleroRestoreExec(ctx context.Context, veleroCLI, veleroNamespace, restoreName string, args []string, phaseExpect velerov1api.RestorePhase) error {
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}

	return checkRestorePhase(ctx, veleroCLI, veleroNamespace, restoreName, phaseExpect)
}

func VeleroBackupExec(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string, args []string) error {
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	return checkBackupPhase(ctx, veleroCLI, veleroNamespace, backupName, velerov1api.BackupPhaseCompleted)
}

func VeleroBackupDelete(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string) error {
	args := []string{"--namespace", veleroNamespace, "delete", "backup", backupName, "--confirm"}
	return VeleroCmdExec(ctx, veleroCLI, args)
}

func VeleroRestoreDelete(ctx context.Context, veleroCLI string, veleroNamespace string, restoreName string) error {
	args := []string{"--namespace", veleroNamespace, "delete", "restore", restoreName, "--confirm"}
	return VeleroCmdExec(ctx, veleroCLI, args)
}

func VeleroScheduleDelete(ctx context.Context, veleroCLI string, veleroNamespace string, scheduleName string) error {
	args := []string{"--namespace", veleroNamespace, "delete", "schedule", scheduleName, "--confirm"}
	return VeleroCmdExec(ctx, veleroCLI, args)
}

func VeleroScheduleCreate(ctx context.Context, veleroCLI string, veleroNamespace string, scheduleName string, args []string) error {
	args = append([]string{
		"--namespace", veleroNamespace, "create", "schedule", scheduleName,
	}, args...)
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	return checkSchedulePhase(ctx, veleroCLI, veleroNamespace, scheduleName)
}

func VeleroSchedulePause(ctx context.Context, veleroCLI string, veleroNamespace string, scheduleName string) error {
	var args []string
	args = []string{
		"--namespace", veleroNamespace, "schedule", "pause", scheduleName,
	}
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	return checkSchedulePause(ctx, veleroCLI, veleroNamespace, scheduleName, true)
}

func VeleroScheduleUnpause(ctx context.Context, veleroCLI string, veleroNamespace string, scheduleName string) error {
	var args []string
	args = []string{
		"--namespace", veleroNamespace, "schedule", "unpause", scheduleName,
	}
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	return checkSchedulePause(ctx, veleroCLI, veleroNamespace, scheduleName, false)
}

func VeleroCmdExec(ctx context.Context, veleroCLI string, args []string) error {
	cmd := exec.CommandContext(ctx, veleroCLI, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("velero cmd =%v\n", cmd)
	err := cmd.Run()
	if strings.Contains(fmt.Sprint(cmd.Stdout), "Failed") {
		return errors.New(fmt.Sprintf("velero cmd =%v return with failure\n", cmd))
	}
	return err
}

func VeleroBackupLogs(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string) error {
	args := []string{
		"--namespace", veleroNamespace, "backup", "describe", backupName,
	}
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	args = []string{
		"--namespace", veleroNamespace, "backup", "logs", backupName,
	}
	return VeleroCmdExec(ctx, veleroCLI, args)
}

func RunDebug(ctx context.Context, veleroCLI, veleroNamespace, backup, restore string) {
	output := fmt.Sprintf("debug-bundle-%d.tar.gz", time.Now().UnixNano())
	args := []string{"debug", "--namespace", veleroNamespace, "--output", output, "--verbose"}
	if len(backup) > 0 {
		args = append(args, "--backup", backup)
	}
	if len(restore) > 0 {
		//args = append(args, "--restore", restore)
	}
	fmt.Printf("Generating the debug tarball at %s\n", output)
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		fmt.Println(errors.Wrapf(err, "failed to run the debug command"))
	}
}

func VeleroCreateBackupLocation(ctx context.Context,
	veleroCLI,
	veleroNamespace,
	name,
	objectStoreProvider,
	bucket,
	prefix,
	config,
	secretName,
	secretKey string,
) error {
	args := []string{
		"--namespace", veleroNamespace,
		"create", "backup-location", name,
		"--provider", objectStoreProvider,
		"--bucket", bucket,
	}

	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}

	if config != "" {
		args = append(args, "--config", config)
	}

	if secretName != "" && secretKey != "" {
		args = append(args, "--credential", fmt.Sprintf("%s=%s", secretName, secretKey))
	}
	return VeleroCmdExec(ctx, veleroCLI, args)
}

func VeleroVersion(ctx context.Context, veleroCLI, veleroNamespace string) error {
	args := []string{
		"version", "--namespace", veleroNamespace,
	}
	if err := VeleroCmdExec(ctx, veleroCLI, args); err != nil {
		return err
	}
	return nil
}

func getProviderPlugins(ctx context.Context, veleroCLI, objectStoreProvider, providerPlugins, feature string) ([]string, error) {
	// Fetch the plugins for the provider before checking for the object store provider below.
	var plugins []string
	if len(providerPlugins) > 0 {
		plugins = strings.Split(providerPlugins, ",")
	} else {
		version, err := getVeleroVersion(ctx, veleroCLI, true)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to get velero version")
		}
		plugins, err = GetProviderPluginsByVersion(version, objectStoreProvider, feature)
		if err != nil {
			return nil, errors.WithMessagef(err, "Fail to get plugin by provider %s and version %s", objectStoreProvider, version)
		}
	}
	return plugins, nil
}

// VeleroAddPluginsForProvider determines which plugins need to be installed for a provider and
// installs them in the current Velero installation, skipping over those that are already installed.
func VeleroAddPluginsForProvider(ctx context.Context, veleroCLI string, veleroNamespace string, provider string, addPlugins, feature string) error {
	plugins, err := getProviderPlugins(ctx, veleroCLI, provider, addPlugins, feature)
	fmt.Printf("addPlugins cmd =%v\n", addPlugins)
	fmt.Printf("provider cmd = %v\n", provider)
	fmt.Printf("plugins cmd = %v\n", plugins)
	if err != nil {
		return errors.WithMessage(err, "Failed to get plugins")
	}
	for _, plugin := range plugins {
		stdoutBuf := new(bytes.Buffer)
		stderrBuf := new(bytes.Buffer)

		installPluginCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "plugin", "add", plugin)
		fmt.Printf("installPluginCmd cmd =%v\n", installPluginCmd)
		installPluginCmd.Stdout = stdoutBuf
		installPluginCmd.Stderr = stderrBuf

		err := installPluginCmd.Run()

		fmt.Fprint(os.Stdout, stdoutBuf)
		fmt.Fprint(os.Stderr, stderrBuf)

		if err != nil {
			// If the plugin failed to install as it was already installed, ignore the error and continue
			// TODO: Check which plugins are already installed by inspecting `velero plugin get`
			if !strings.Contains(stderrBuf.String(), "Duplicate value") {
				return errors.WithMessagef(err, "error installing plugin %s", plugin)
			}
		}
	}

	return nil
}

// WaitForVSphereUploadCompletion waits for uploads started by the Velero Plug-in for vSphere to complete
// TODO - remove after upload progress monitoring is implemented
func WaitForVSphereUploadCompletion(ctx context.Context, timeout time.Duration, namespace string, expectCount int) error {
	err := wait.PollImmediate(time.Second*5, timeout, func() (bool, error) {
		checkSnapshotCmd := exec.CommandContext(ctx, "kubectl",
			"get", "-n", namespace, "snapshots.backupdriver.cnsdp.vmware.com", "-o=jsonpath='{range .items[*]}{.spec.resourceHandle.name}{\"=\"}{.status.phase}{\"\\n\"}{end}'")
		fmt.Printf("checkSnapshotCmd cmd =%v\n", checkSnapshotCmd)
		stdout, stderr, err := veleroexec.RunCommand(checkSnapshotCmd)
		if err != nil {
			fmt.Print(stdout)
			fmt.Print(stderr)
			return false, errors.Wrap(err, "failed to wait for vSphere upload completion")
		}
		lines := strings.Split(stdout, "\n")
		complete := true
		actualCount := 0

		for _, curLine := range lines {
			fmt.Println(curLine)
			comps := strings.Split(curLine, "=")
			// SnapshotPhase represents the lifecycle phase of a Snapshot.
			// New - No work yet, next phase is InProgress
			// InProgress - snapshot being taken
			// Snapshotted - local snapshot complete, next phase is Protecting or SnapshotFailed
			// SnapshotFailed - end state, snapshot was not able to be taken
			// Uploading - snapshot is being moved to durable storage
			// Uploaded - end state, snapshot has been protected
			// UploadFailed - end state, unable to move to durable storage
			// Canceling - when the SanpshotCancel flag is set, if the Snapshot has not already moved into a terminal state, the
			//             status will move to Canceling.  The snapshot ID will be removed from the status status if has been filled in
			//             and the snapshot ID will not longer be valid for a Clone operation
			// Canceled - the operation was canceled, the snapshot ID is not valid
			// Canceled - the operation was canceled, the snapshot ID is not valid
			if len(comps) == 2 {
				phase := comps[1]
				actualCount++
				switch phase {
				case "Uploaded":
				case "New", "InProgress", "Snapshotted", "Uploading":
					complete = false
				default:
					return false, fmt.Errorf("unexpected snapshot phase: %s", phase)
				}
			}
		}

		if expectCount != actualCount {
			fmt.Printf("Snapshot expect count and actual count: %d-%d", expectCount, actualCount)
			complete = false
			if expectCount == 0 {
				return true, nil
			}
		}
		return complete, nil
	})

	return err
}

func GetVsphereSnapshotIDs(ctx context.Context, timeout time.Duration, namespace string, podNameList []string) ([]string, error) {
	checkSnapshotCmd := exec.CommandContext(ctx, "kubectl",
		"get", "-n", namespace, "snapshots.backupdriver.cnsdp.vmware.com", "-o=jsonpath='{range .items[*]}{.spec.resourceHandle.name}{\"=\"}{.status.snapshotID}{\"\\n\"}{end}'")
	fmt.Printf("checkSnapshotCmd cmd =%v\n", checkSnapshotCmd)
	stdout, stderr, err := veleroexec.RunCommand(checkSnapshotCmd)
	if err != nil {
		fmt.Print(stdout)
		fmt.Print(stderr)
		return nil, errors.Wrap(err, "Failed to get vSphere snapshot ID list from snapshots.backupdriver.cnsdp.vmware.com")
	}
	stdout = strings.Replace(stdout, "'", "", -1)
	lines := strings.Split(stdout, "\n")
	var result []string
	for _, curLine := range lines {
		fmt.Println("curLine:" + curLine)
		curLine = strings.Replace(curLine, "\n", "", -1)
		if len(curLine) == 0 {
			continue
		}
		var Exist bool
		for _, podName := range podNameList {
			if podName != "" && strings.Contains(curLine, podName) {
				Exist = true
				break
			}
		}
		if !Exist {
			continue
		}
		snapshotID := curLine[strings.LastIndex(curLine, ":")+1:]
		fmt.Println("snapshotID:" + snapshotID)
		snapshotIDDec, _ := b64.StdEncoding.DecodeString(snapshotID)
		fmt.Println("snapshotIDDec:" + string(snapshotIDDec))
		result = append(result, string(snapshotIDDec))
		fmt.Println(result)
	}
	fmt.Println(result)
	return result, nil
}

func getVeleroVersion(ctx context.Context, veleroCLI string, clientOnly bool) (string, error) {
	args := []string{"version", "--timeout", "60s"}
	if clientOnly {
		args = append(args, "--client-only")
	}
	cmd := exec.CommandContext(ctx, veleroCLI, args...)
	fmt.Println("Get Version Command:" + cmd.String())
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get velero version, stdout=%s, stderr=%s", stdout, stderr)
	}

	output := strings.Replace(stdout, "\n", " ", -1)
	fmt.Println("Version:" + output)
	resultCount := 3
	regexpRule := `(?i)client\s*:\s*version\s*:\s*(\S+).+server\s*:\s*version\s*:\s*(\S+)`
	if clientOnly {
		resultCount = 2
		regexpRule = `(?i)client\s*:\s*version\s*:\s*(\S+)`
	}
	regCompiler := regexp.MustCompile(regexpRule)
	versionMatches := regCompiler.FindStringSubmatch(output)
	if len(versionMatches) != resultCount {
		return "", errors.New("failed to parse velero version from output")
	}
	if !clientOnly {
		if versionMatches[1] != versionMatches[2] {
			return "", errors.New("velero server and client version are not matched")
		}
	}
	return versionMatches[1], nil
}

func CheckVeleroVersion(ctx context.Context, veleroCLI string, expectedVer string) error {
	tag := expectedVer
	tagInstalled, err := getVeleroVersion(ctx, veleroCLI, false)
	if err != nil {
		return errors.WithMessagef(err, "failed to get Velero version")
	}
	if strings.Trim(tag, " ") != strings.Trim(tagInstalled, " ") {
		return errors.New(fmt.Sprintf("velero version %s is not as expected %s", tagInstalled, tag))
	}
	fmt.Printf("Velero version %s is as expected %s\n", tagInstalled, tag)
	return nil
}

func InstallVeleroCLI(version string) (string, error) {
	var tempVeleroCliDir string
	name := "velero-" + version + "-" + runtime.GOOS + "-" + runtime.GOARCH
	postfix := ".tar.gz"
	tarball := name + postfix
	err := wait.PollImmediate(time.Second*5, time.Minute*5, func() (bool, error) {
		tempFile, err := getVeleroCliTarball("https://github.com/vmware-tanzu/velero/releases/download/" + version + "/" + tarball)
		if err != nil {
			return false, errors.WithMessagef(err, "failed to get Velero CLI tarball")
		}
		tempVeleroCliDir, err = os.MkdirTemp("", "velero-test")
		if err != nil {
			return false, errors.WithMessagef(err, "failed to create temp dir for tarball extraction")
		}

		cmd := exec.Command("tar", "-xvf", tempFile.Name(), "-C", tempVeleroCliDir)
		defer os.Remove(tempFile.Name())

		if _, err := cmd.Output(); err != nil {
			return false, errors.WithMessagef(err, "failed to extract file from velero CLI tarball")
		}
		return true, nil
	})
	if err != nil {
		return "", errors.WithMessagef(err, "failed to install velero CLI")
	}
	return tempVeleroCliDir + "/" + name + "/velero", nil
}

func getVeleroCliTarball(cliTarballUrl string) (*os.File, error) {
	lastInd := strings.LastIndex(cliTarballUrl, "/")
	tarball := cliTarballUrl[lastInd+1:]

	resp, err := http.Get(cliTarballUrl)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to access Velero CLI tarball")
	}
	defer resp.Body.Close()

	tarballBuf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read buffer for tarball %s.", tarball)
	}
	tmpfile, err := os.CreateTemp("", tarball)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to create temp file for tarball %s locally.", tarball)
	}

	if _, err := tmpfile.Write(tarballBuf); err != nil {
		return nil, errors.WithMessagef(err, "failed to write tarball file %s locally.", tarball)
	}

	return tmpfile, nil
}

func DeleteBackupResource(ctx context.Context, veleroCLI string, backupName string) error {
	args := []string{"backup", "delete", backupName, "--confirm"}

	cmd := exec.CommandContext(ctx, veleroCLI, args...)
	fmt.Println("Delete backup Command:" + cmd.String())
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "Fail to get delete backup, stdout=%s, stderr=%s", stdout, stderr)
	}

	output := strings.Replace(stdout, "\n", " ", -1)
	fmt.Println("Backup delete command output:" + output)

	args = []string{"backup", "get", backupName}

	retryTimes := 5
	for i := 1; i < retryTimes+1; i++ {
		cmd = exec.CommandContext(ctx, veleroCLI, args...)
		fmt.Printf("Try %d times to delete backup %s \n", i, cmd.String())
		stdout, stderr, err = veleroexec.RunCommand(cmd)
		if err != nil {
			if strings.Contains(stderr, "not found") {
				fmt.Printf("|| EXPECTED || - Backup %s was deleted successfully according to message %s\n", backupName, stderr)
				return nil
			}
			return errors.Wrapf(err, "Fail to perform get backup, stdout=%s, stderr=%s", stdout, stderr)
		}
		time.Sleep(1 * time.Minute)
	}
	return nil
}

func GetBackup(ctx context.Context, veleroCLI string, backupName string) (string, string, error) {
	args := []string{"backup", "get", backupName}
	cmd := exec.CommandContext(ctx, veleroCLI, args...)
	return veleroexec.RunCommand(cmd)
}

func IsBackupExist(ctx context.Context, veleroCLI string, backupName string) (bool, error) {
	out, outerr, err := GetBackup(ctx, veleroCLI, backupName)
	if err != nil {
		if strings.Contains(outerr, "not found") {
			fmt.Printf("Backup CR %s was not found\n", backupName)
			return false, nil
		}
		return false, err
	}
	fmt.Printf("Backup <%s> exist locally according to output \n[%s]\n", backupName, out)
	return true, nil
}

func WaitBackupDeleted(ctx context.Context, veleroCLI string, backupName string, timeout time.Duration) error {
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		if exist, err := IsBackupExist(ctx, veleroCLI, backupName); err != nil {
			return false, err
		} else {
			if exist {
				return false, nil
			} else {
				fmt.Printf("Backup %s does not exist\n", backupName)
				return true, nil
			}
		}
	})
}

func WaitForExpectedStateOfBackup(ctx context.Context, veleroCLI string, backupName string,
	timeout time.Duration, existing bool) error {
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		if exist, err := IsBackupExist(ctx, veleroCLI, backupName); err != nil {
			return false, err
		} else {
			msg := "does not exist as expect"
			if exist {
				msg = "was found as expect"
			}
			if exist == existing {
				fmt.Println("Backup <" + backupName + "> " + msg)
				return true, nil
			} else {
				fmt.Println("Backup <" + backupName + "> " + msg)
				return false, nil
			}
		}
	})
}

func WaitForBackupToBeCreated(ctx context.Context, veleroCLI string, backupName string, timeout time.Duration) error {
	return WaitForExpectedStateOfBackup(ctx, veleroCLI, backupName, timeout, true)
}

func WaitForBackupToBeDeleted(ctx context.Context, veleroCLI string, backupName string, timeout time.Duration) error {
	return WaitForExpectedStateOfBackup(ctx, veleroCLI, backupName, timeout, false)
}

func WaitForBackupsToBeDeleted(ctx context.Context, veleroCLI string, backups []string, timeout time.Duration) error {
	var err error
	for _, backupName := range backups {
		fmt.Println("Waiting for deletion of backup <" + backupName + ">")
		err = WaitForExpectedStateOfBackup(ctx, veleroCLI, backupName, timeout, false)
		if err != nil {
			return err
		}
	}
	fmt.Println("All backups were deleted.")
	return nil
}

func GetBackupsFromBsl(ctx context.Context, veleroCLI, bslName string) ([]string, error) {
	args1 := []string{"get", "backups"}
	if strings.TrimSpace(bslName) != "" {
		args1 = append(args1, "-l", "velero.io/storage-location="+bslName)
	}
	cmds := []*common.OsCommandLine{}

	cmd := &common.OsCommandLine{
		Cmd:  veleroCLI,
		Args: args1,
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "tail",
		Args: []string{"-n", "+2"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetScheduledBackupsCreationTime(ctx context.Context, veleroCLI, bslName, scheduleName string) ([]string, error) {
	var creationTimes []string
	backups, err := GetBackupsCreationTime(ctx, veleroCLI, bslName)
	if err != nil {
		return nil, err
	}
	for _, b := range backups {
		if strings.Contains(b, scheduleName) {
			creationTimes = append(creationTimes, b)
		}
	}
	return creationTimes, nil
}
func GetBackupsCreationTime(ctx context.Context, veleroCLI, bslName string) ([]string, error) {
	args1 := []string{"get", "backups"}
	createdTime := "$1,\",\" $5,$6,$7,$8"
	if strings.TrimSpace(bslName) != "" {
		args1 = append(args1, "-l", "velero.io/storage-location="+bslName)
	}
	cmds := []*common.OsCommandLine{}

	cmd := &common.OsCommandLine{
		Cmd:  veleroCLI,
		Args: args1,
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print " + createdTime + "}"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "tail",
		Args: []string{"-n", "+2"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetAllBackups(ctx context.Context, veleroCLI string) ([]string, error) {
	return GetBackupsFromBsl(ctx, veleroCLI, "")
}
func DeleteBslResource(ctx context.Context, veleroCLI string, bslName string) error {
	args := []string{"backup-location", "delete", bslName, "--confirm"}

	cmd := exec.CommandContext(ctx, veleroCLI, args...)
	fmt.Println("Delete backup location Command:" + cmd.String())
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	if err != nil {
		return errors.Wrapf(err, "Fail to get delete location, stdout=%s, stderr=%s", stdout, stderr)
	}

	output := strings.Replace(stdout, "\n", " ", -1)
	fmt.Println("Backup location delete command output:" + output)

	fmt.Println(stdout)
	fmt.Println(stderr)
	return nil
}

func SnapshotCRsCountShouldBe(ctx context.Context, namespace, backupName string, expectedCount int) error {

	checkSnapshotCmd := exec.CommandContext(ctx, "kubectl",
		"get", "-n", namespace, "snapshots.backupdriver.cnsdp.vmware.com", "-o=jsonpath='{range .items[*]}{.metadata.labels.velero\\.io\\/backup-name}{\"\\n\"}{end}'")
	fmt.Printf("checkSnapshotCmd cmd =%v\n", checkSnapshotCmd)
	stdout, stderr, err := veleroexec.RunCommand(checkSnapshotCmd)
	if err != nil {
		fmt.Print(stdout)
		fmt.Print(stderr)
		return errors.Wrap(err, fmt.Sprintf("Failed getting snapshot CR of backup %s in namespace %d", backupName, expectedCount))
	}
	count := 0
	stdout = strings.Replace(stdout, "'", "", -1)
	arr := strings.Split(stdout, "\n")
	for _, bn := range arr {
		fmt.Println("Snapshot CR:" + bn)
		if strings.Contains(bn, backupName) {
			count++
		}
	}
	if count == expectedCount {
		return nil
	} else {
		return errors.New(fmt.Sprintf("SnapshotCR count %d of backup %s in namespace %s is not as expected %d", count, backupName, namespace, expectedCount))
	}
}

func BackupRepositoriesCountShouldBe(ctx context.Context, veleroNamespace, targetNamespace string, expectedCount int) error {
	resticArr, err := GetResticRepositories(ctx, veleroNamespace, targetNamespace)
	if err != nil {
		return errors.Wrapf(err, "Fail to get BackupRepositories")
	}
	if len(resticArr) == expectedCount {
		return nil
	} else {
		return errors.New(fmt.Sprintf("BackupRepositories count %d in namespace %s is not as expected %d", len(resticArr), targetNamespace, expectedCount))
	}
}

func GetResticRepositories(ctx context.Context, veleroNamespace, targetNamespace string) ([]string, error) {
	cmds := []*common.OsCommandLine{}
	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", "-n", veleroNamespace, "BackupRepositories"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "grep",
		Args: []string{targetNamespace},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetSnapshotCheckPoint(client TestClient, VeleroCfg VeleroConfig, expectCount int, namespaceBackedUp, backupName string, kibishiiPodNameList []string) (SnapshotCheckPoint, error) {
	var snapshotCheckPoint SnapshotCheckPoint
	var err error
	snapshotCheckPoint.ExpectCount = expectCount
	snapshotCheckPoint.NamespaceBackedUp = namespaceBackedUp
	snapshotCheckPoint.PodName = kibishiiPodNameList
	if VeleroCfg.CloudProvider == "azure" && strings.EqualFold(VeleroCfg.Features, "EnableCSI") {
		snapshotCheckPoint.EnableCSI = true
		if snapshotCheckPoint.SnapshotIDList, err = util.CheckVolumeSnapshotCR(client, backupName, expectCount); err != nil {
			return snapshotCheckPoint, errors.Wrapf(err, "Fail to get Azure CSI snapshot content")
		}
	}
	fmt.Println(snapshotCheckPoint)
	return snapshotCheckPoint, nil
}

func GetBackupTTL(ctx context.Context, veleroNamespace, backupName string) (string, error) {
	checkSnapshotCmd := exec.CommandContext(ctx, "kubectl",
		"get", "backup", "-n", veleroNamespace, backupName, "-o=jsonpath='{.spec.ttl}'")
	fmt.Printf("checkSnapshotCmd cmd =%v\n", checkSnapshotCmd)
	stdout, stderr, err := veleroexec.RunCommand(checkSnapshotCmd)
	if err != nil {
		fmt.Print(stdout)
		fmt.Print(stderr)
		return "", errors.Wrap(err, fmt.Sprintf("failed to run command %s", checkSnapshotCmd))
	}
	return stdout, err
}

func GetVersionList(veleroCli, veleroVersion string) []VeleroCLI2Version {
	var veleroCLI2VersionList []VeleroCLI2Version
	veleroVersionList := strings.Split(veleroVersion, ",")
	veleroCliList := strings.Split(veleroCli, ",")

	for _, veleroVersion := range veleroVersionList {
		veleroCLI2VersionList = append(veleroCLI2VersionList,
			VeleroCLI2Version{veleroVersion, ""})
	}
	for i, veleroCli := range veleroCliList {
		if i == len(veleroVersionList)-1 {
			break
		}
		veleroCLI2VersionList[i].VeleroCLI = veleroCli
	}
	return veleroCLI2VersionList
}
func DeleteBackups(ctx context.Context, client TestClient) error {
	backupList := new(velerov1api.BackupList)
	if err := client.Kubebuilder.List(ctx, backupList, &kbclient.ListOptions{Namespace: VeleroCfg.VeleroNamespace}); err != nil {
		return fmt.Errorf("failed to list backup object in %s namespace with err %v", VeleroCfg.VeleroNamespace, err)
	}
	for _, backup := range backupList.Items {
		fmt.Printf("Backup %s is going to be deleted...\n", backup.Name)
		if err := VeleroBackupDelete(ctx, VeleroCfg.VeleroCLI, VeleroCfg.VeleroNamespace, backup.Name); err != nil {
			return err
		}
	}
	return nil
}

func GetSchedule(ctx context.Context, veleroNamespace, scheduleName string) (string, error) {
	checkSnapshotCmd := exec.CommandContext(ctx, "kubectl",
		"get", "schedule", "-n", veleroNamespace, scheduleName, "-o=jsonpath='{.metadata.creationTimestamp}'")
	fmt.Printf("Cmd =%v\n", checkSnapshotCmd)
	stdout, stderr, err := veleroexec.RunCommand(checkSnapshotCmd)
	if err != nil {
		fmt.Print(stdout)
		fmt.Print(stderr)
		return "", errors.Wrap(err, fmt.Sprintf("failed to run command %s", checkSnapshotCmd))
	}
	return stdout, err
}

func VeleroUpgrade(ctx context.Context, veleroCfg VeleroConfig) error {
	crd, err := ApplyCRDs(ctx, veleroCfg.VeleroCLI)
	if err != nil {
		return errors.Wrap(err, "Fail to Apply CRDs")
	}
	fmt.Println(crd)
	deploy, err := UpdateVeleroDeployment(ctx, veleroCfg)
	if err != nil {
		return errors.Wrap(err, "Fail to update Velero deployment")
	}
	fmt.Println(deploy)
	if veleroCfg.UseNodeAgent {
		dsjson, err := KubectlGetDsJson(veleroCfg.VeleroNamespace)
		if err != nil {
			return errors.Wrap(err, "Fail to update Velero deployment")
		}

		err = DeleteVeleroDs(ctx)
		if err != nil {
			return errors.Wrap(err, "Fail to delete Velero ds")
		}
		update, err := UpdateNodeAgent(ctx, veleroCfg, dsjson)
		fmt.Println(update)
		if err != nil {
			return errors.Wrap(err, "Fail to update node agent")
		}
	}
	return waitVeleroReady(ctx, veleroCfg.VeleroNamespace, veleroCfg.UseNodeAgent)
}
func ApplyCRDs(ctx context.Context, veleroCLI string) ([]string, error) {
	cmds := []*common.OsCommandLine{}

	cmd := &common.OsCommandLine{
		Cmd:  veleroCLI,
		Args: []string{"install", "--crds-only", "--dry-run", "-o", "yaml"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"apply", "-f", "-"},
	}
	cmds = append(cmds, cmd)
	return common.GetListByCmdPipes(ctx, cmds)
}

func UpdateVeleroDeployment(ctx context.Context, veleroCfg VeleroConfig) ([]string, error) {
	cmds := []*common.OsCommandLine{}

	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", "deploy", "-n", veleroCfg.VeleroNamespace, "-ojson"},
	}
	cmds = append(cmds, cmd)
	var args string
	if veleroCfg.CloudProvider == "vsphere" {
		args = fmt.Sprintf("s#\\\"image\\\"\\: \\\"velero\\/velero\\:v[0-9]*.[0-9]*.[0-9]\\\"#\\\"image\\\"\\: \\\"harbor-repo.vmware.com\\/velero_ci\\/velero\\:%s\\\"#g", veleroCfg.VeleroVersion)
	} else {
		args = fmt.Sprintf("s#\\\"image\\\"\\: \\\"velero\\/velero\\:v[0-9]*.[0-9]*.[0-9]\\\"#\\\"image\\\"\\: \\\"gcr.io\\/velero-gcp\\/nightly\\/velero\\:%s\\\"#g", veleroCfg.VeleroVersion)
	}
	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{args},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{fmt.Sprintf("s#\\\"server\\\",#\\\"server\\\",\\\"--uploader-type=%s\\\",#g", veleroCfg.UploaderType)},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{"s#default-volumes-to-restic#default-volumes-to-fs-backup#g"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{"s#default-restic-prune-frequency#default-repo-maintain-frequency#g"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{"s#restic-timeout#fs-backup-timeout#g"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"apply", "-f", "-"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func UpdateNodeAgent(ctx context.Context, veleroCfg VeleroConfig, dsjson string) ([]string, error) {
	cmds := []*common.OsCommandLine{}

	cmd := &common.OsCommandLine{
		Cmd:  "echo",
		Args: []string{dsjson},
	}
	cmds = append(cmds, cmd)
	var args string
	if veleroCfg.CloudProvider == "vsphere" {
		args = fmt.Sprintf("s#\\\"image\\\"\\: \\\"velero\\/velero\\:v[0-9]*.[0-9]*.[0-9]\\\"#\\\"image\\\"\\: \\\"harbor-repo.vmware.com\\/velero_ci\\/velero\\:%s\\\"#g", veleroCfg.VeleroVersion)
	} else {
		args = fmt.Sprintf("s#\\\"image\\\"\\: \\\"velero\\/velero\\:v[0-9]*.[0-9]*.[0-9]\\\"#\\\"image\\\"\\: \\\"gcr.io\\/velero-gcp\\/nightly\\/velero\\:%s\\\"#g", veleroCfg.VeleroVersion)
	}
	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{args},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{"s#\\\"name\\\"\\: \\\"restic\\\"#\\\"name\\\"\\: \\\"node-agent\\\"#g"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "sed",
		Args: []string{"s#\\\"restic\\\",#\\\"node-agent\\\",#g"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"create", "-f", "-"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetVeleroResource(ctx context.Context, veleroNamespace, namespace, resourceName string) ([]string, error) {
	cmds := []*common.OsCommandLine{}
	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", resourceName, "-n", veleroNamespace},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "grep",
		Args: []string{namespace},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetPVB(ctx context.Context, veleroNamespace, namespace string) ([]string, error) {
	return GetVeleroResource(ctx, veleroNamespace, namespace, "podvolumebackup")
}

func GetPVR(ctx context.Context, veleroNamespace, namespace string) ([]string, error) {
	return GetVeleroResource(ctx, veleroNamespace, namespace, "podvolumerestore")
}

func IsSupportUploaderType(version string) (bool, error) {
	if strings.Contains(version, "self") {
		return true, nil
	}
	verSupportUploaderType, err := ver.ParseSemantic("v1.10.0")
	if err != nil {
		return false, err
	}
	v, err := ver.ParseSemantic(version)
	if err != nil {
		return false, err
	}
	if v.AtLeast(verSupportUploaderType) {
		return true, nil
	} else {
		return false, nil
	}
}
