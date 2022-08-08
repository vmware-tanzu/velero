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

package restic

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	repoconfig "github.com/vmware-tanzu/velero/pkg/repository/config"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

const (
	// DaemonSet is the name of the Velero restic daemonset.
	DaemonSet = "restic"

	// InitContainer is the name of the init container added
	// to workload pods to help with restores.
	InitContainer = "restic-wait"

	// DefaultMaintenanceFrequency is the default time interval
	// at which restic prune is run.
	DefaultMaintenanceFrequency = 7 * 24 * time.Hour

	// DefaultVolumesToRestic specifies whether restic should be used, by default, to
	// take backup of all pod volumes.
	DefaultVolumesToRestic = false
)

// SnapshotIdentifier uniquely identifies a restic snapshot
// taken by Velero.
type SnapshotIdentifier struct {
	// VolumeNamespace is the namespace of the pod/volume that
	// the restic snapshot is for.
	VolumeNamespace string

	// BackupStorageLocation is the backup's storage location
	// name.
	BackupStorageLocation string

	// SnapshotID is the short ID of the restic snapshot.
	SnapshotID string
}

// GetSnapshotsInBackup returns a list of all restic snapshot ids associated with
// a given Velero backup.
func GetSnapshotsInBackup(ctx context.Context, backup *velerov1api.Backup, kbClient client.Client) ([]SnapshotIdentifier, error) {
	podVolumeBackups := &velerov1api.PodVolumeBackupList{}
	options := &client.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			velerov1api.BackupNameLabel: label.GetValidName(backup.Name),
		}).AsSelector(),
	}

	err := kbClient.List(ctx, podVolumeBackups, options)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var res []SnapshotIdentifier
	for _, item := range podVolumeBackups.Items {
		if item.Status.SnapshotID == "" {
			continue
		}
		res = append(res, SnapshotIdentifier{
			VolumeNamespace:       item.Spec.Pod.Namespace,
			BackupStorageLocation: backup.Spec.StorageLocation,
			SnapshotID:            item.Status.SnapshotID,
		})
	}

	return res, nil
}

// TempCACertFile creates a temp file containing a CA bundle
// and returns its path. The caller should generally call os.Remove()
// to remove the file when done with it.
func TempCACertFile(caCert []byte, bsl string, fs filesystem.Interface) (string, error) {
	file, err := fs.TempFile("", fmt.Sprintf("cacert-%s", bsl))
	if err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := file.Write(caCert); err != nil {
		// nothing we can do about an error closing the file here, and we're
		// already returning an error about the write failing.
		file.Close()
		return "", errors.WithStack(err)
	}

	name := file.Name()

	if err := file.Close(); err != nil {
		return "", errors.WithStack(err)
	}

	return name, nil
}

// NewPodVolumeRestoreListOptions creates a ListOptions with a label selector configured to
// find PodVolumeRestores for the restore identified by name.
func NewPodVolumeRestoreListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", velerov1api.RestoreNameLabel, label.GetValidName(name)),
	}
}

// CmdEnv returns a list of environment variables (in the format var=val) that
// should be used when running a restic command for a particular backend provider.
// This list is the current environment, plus any provider-specific variables restic needs.
func CmdEnv(backupLocation *velerov1api.BackupStorageLocation, credentialFileStore credentials.FileStore) ([]string, error) {
	env := os.Environ()
	customEnv := map[string]string{}
	var err error

	config := backupLocation.Spec.Config
	if config == nil {
		config = map[string]string{}
	}

	if backupLocation.Spec.Credential != nil {
		credsFile, err := credentialFileStore.Path(backupLocation.Spec.Credential)
		if err != nil {
			return []string{}, errors.WithStack(err)
		}
		config[repoconfig.CredentialsFileKey] = credsFile
	}

	backendType := repoconfig.GetBackendType(backupLocation.Spec.Provider)

	switch backendType {
	case repoconfig.AWSBackend:
		customEnv, err = repoconfig.GetS3ResticEnvVars(config)
		if err != nil {
			return []string{}, err
		}
	case repoconfig.AzureBackend:
		customEnv, err = repoconfig.GetAzureResticEnvVars(config)
		if err != nil {
			return []string{}, err
		}
	case repoconfig.GCPBackend:
		customEnv, err = repoconfig.GetGCPResticEnvVars(config)
		if err != nil {
			return []string{}, err
		}
	}

	for k, v := range customEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env, nil
}
