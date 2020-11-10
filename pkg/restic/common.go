/*
Copyright 2018 the Velero contributors.

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
	"strings"
	"time"

	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
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

	// PVCNameAnnotation is the key for the annotation added to
	// pod volume backups when they're for a PVC.
	PVCNameAnnotation = "velero.io/pvc-name"

	// VolumesToBackupAnnotation is the annotation on a pod whose mounted volumes
	// need to be backed up using restic.
	VolumesToBackupAnnotation = "backup.velero.io/backup-volumes"

	// VolumesToExcludeAnnotation is the annotation on a pod whose mounted volumes
	// should be excluded from restic backup.
	VolumesToExcludeAnnotation = "backup.velero.io/backup-volumes-excludes"

	// Deprecated.
	//
	// TODO(2.0): remove
	podAnnotationPrefix = "snapshot.velero.io/"
)

// getPodSnapshotAnnotations returns a map, of volume name -> snapshot id,
// of all restic snapshots for this pod.
// TODO(2.0) to remove
// Deprecated: we will stop using pod annotations to record restic snapshot IDs after they're taken,
// therefore we won't need to check if these annotations exist.
func getPodSnapshotAnnotations(obj metav1.Object) map[string]string {
	var res map[string]string

	insertSafe := func(k, v string) {
		if res == nil {
			res = make(map[string]string)
		}
		res[k] = v
	}

	for k, v := range obj.GetAnnotations() {
		if strings.HasPrefix(k, podAnnotationPrefix) {
			insertSafe(k[len(podAnnotationPrefix):], v)
		}
	}

	return res
}

func isPVBMatchPod(pvb *velerov1api.PodVolumeBackup, pod metav1.Object) bool {
	return pod.GetName() == pvb.Spec.Pod.Name && pod.GetNamespace() == pvb.Spec.Pod.Namespace
}

// GetVolumeBackupsForPod returns a map, of volume name -> snapshot id,
// of the PodVolumeBackups that exist for the provided pod.
func GetVolumeBackupsForPod(podVolumeBackups []*velerov1api.PodVolumeBackup, pod metav1.Object) map[string]string {
	volumes := make(map[string]string)

	for _, pvb := range podVolumeBackups {
		if !isPVBMatchPod(pvb, pod) {
			continue
		}

		// skip PVBs without a snapshot ID since there's nothing
		// to restore (they could be failed, or for empty volumes).
		if pvb.Status.SnapshotID == "" {
			continue
		}

		volumes[pvb.Spec.Volume] = pvb.Status.SnapshotID
	}

	if len(volumes) > 0 {
		return volumes
	}

	return getPodSnapshotAnnotations(pod)
}

// GetVolumesToBackup returns a list of volume names to backup for
// the provided pod.
// Deprecated: Use GetPodVolumesUsingRestic instead.
func GetVolumesToBackup(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	backupsValue := annotations[VolumesToBackupAnnotation]
	if backupsValue == "" {
		return nil
	}

	return strings.Split(backupsValue, ",")
}

func getVolumesToExclude(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	return strings.Split(annotations[VolumesToExcludeAnnotation], ",")
}

func contains(list []string, k string) bool {
	for _, i := range list {
		if i == k {
			return true
		}
	}
	return false
}

// GetPodVolumesUsingRestic returns a list of volume names to backup for the provided pod.
func GetPodVolumesUsingRestic(pod *corev1api.Pod, defaultVolumesToRestic bool) []string {
	if !defaultVolumesToRestic {
		return GetVolumesToBackup(pod)
	}

	volsToExclude := getVolumesToExclude(pod)
	podVolumes := []string{}
	for _, pv := range pod.Spec.Volumes {
		// cannot backup hostpath volumes as they are not mounted into /var/lib/kubelet/pods
		// and therefore not accessible to the restic daemon set.
		if pv.HostPath != nil {
			continue
		}
		// don't backup volumes mounting secrets. Secrets will be backed up separately.
		if pv.Secret != nil {
			continue
		}
		// don't backup volumes mounting config maps. Config maps will be backed up separately.
		if pv.ConfigMap != nil {
			continue
		}
		// don't backup volumes that are included in the exclude list.
		if contains(volsToExclude, pv.Name) {
			continue
		}
		// don't include volumes that mount the default service account token.
		if strings.HasPrefix(pv.Name, "default-token") {
			continue
		}
		podVolumes = append(podVolumes, pv.Name)
	}
	return podVolumes
}

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
func GetSnapshotsInBackup(backup *velerov1api.Backup, podVolumeBackupLister velerov1listers.PodVolumeBackupLister) ([]SnapshotIdentifier, error) {
	selector := labels.Set(map[string]string{
		velerov1api.BackupNameLabel: label.GetValidName(backup.Name),
	}).AsSelector()

	podVolumeBackups, err := podVolumeBackupLister.List(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var res []SnapshotIdentifier
	for _, item := range podVolumeBackups {
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

// TempCredentialsFile creates a temp file containing a restic
// encryption key for the given repo and returns its path. The
// caller should generally call os.Remove() to remove the file
// when done with it.
func TempCredentialsFile(secretLister corev1listers.SecretLister, veleroNamespace, repoName string, fs filesystem.Interface) (string, error) {
	secretGetter := NewListerSecretGetter(secretLister)

	// For now, all restic repos share the same key so we don't need the repoName to fetch it.
	// When we move to full-backup encryption, we'll likely have a separate key per restic repo
	// (all within the Velero server's namespace) so GetRepositoryKey will need to take the repo
	// name as an argument as well.
	repoKey, err := GetRepositoryKey(secretGetter, veleroNamespace)
	if err != nil {
		return "", err
	}

	file, err := fs.TempFile("", fmt.Sprintf("%s-%s", CredentialsSecretName, repoName))
	if err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := file.Write(repoKey); err != nil {
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

func GetCACert(client kbclient.Client, namespace, backupLocation string) ([]byte, error) {
	location := &velerov1api.BackupStorageLocation{}
	if err := client.Get(context.Background(), kbclient.ObjectKey{
		Namespace: namespace,
		Name:      backupLocation,
	}, location); err != nil {
		return nil, err
	}

	if location.Spec.ObjectStorage == nil {
		return nil, nil
	}

	return location.Spec.ObjectStorage.CACert, nil
}

// NewPodVolumeRestoreListOptions creates a ListOptions with a label selector configured to
// find PodVolumeRestores for the restore identified by name.
func NewPodVolumeRestoreListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", velerov1api.RestoreNameLabel, label.GetValidName(name)),
	}
}

// AzureCmdEnv returns a list of environment variables (in the format var=val) that
// should be used when running a restic command for an Azure backend. This list is
// the current environment, plus the Azure-specific variables restic needs, namely
// a storage account name and key.
func AzureCmdEnv(client kbclient.Client, namespace, backupLocation string) ([]string, error) {
	loc := &velerov1api.BackupStorageLocation{}
	if err := client.Get(context.Background(), kbclient.ObjectKey{
		Namespace: namespace,
		Name:      backupLocation,
	}, loc); err != nil {
		return nil, err
	}

	azureVars, err := getAzureResticEnvVars(loc.Spec.Config)
	if err != nil {
		return nil, errors.Wrap(err, "error getting azure restic env vars")
	}

	env := os.Environ()
	for k, v := range azureVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env, nil
}

// S3CmdEnv returns a list of environment variables (in the format var=val) that
// should be used when running a restic command for an S3 backend. This list is
// the current environment, plus the AWS-specific variables restic needs, namely
// a credential profile.
func S3CmdEnv(client kbclient.Client, namespace, backupLocation string) ([]string, error) {
	loc := &velerov1api.BackupStorageLocation{}
	if err := client.Get(context.Background(), kbclient.ObjectKey{
		Namespace: namespace,
		Name:      backupLocation,
	}, loc); err != nil {
		return nil, err
	}

	awsVars, err := getS3ResticEnvVars(loc.Spec.Config)
	if err != nil {
		return nil, errors.Wrap(err, "error getting aws restic env vars")
	}

	env := os.Environ()
	for k, v := range awsVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env, nil
}
