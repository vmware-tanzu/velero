/*
Copyright 2018 the Heptio Ark contributors.

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
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkv1listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/util/filesystem"
)

const (
	DaemonSet                   = "restic"
	InitContainer               = "restic-wait"
	DefaultMaintenanceFrequency = 24 * time.Hour
	ResticLocationConfigKey     = "restic-location"

	podAnnotationPrefix       = "snapshot.ark.heptio.com/"
	volumesToBackupAnnotation = "backup.ark.heptio.com/backup-volumes"
)

// PodHasSnapshotAnnotation returns true if the object has an annotation
// indicating that there is a restic snapshot for a volume in this pod,
// or false otherwise.
func PodHasSnapshotAnnotation(obj metav1.Object) bool {
	for key := range obj.GetAnnotations() {
		if strings.HasPrefix(key, podAnnotationPrefix) {
			return true
		}
	}

	return false
}

// GetPodSnapshotAnnotations returns a map, of volume name -> snapshot id,
// of all restic snapshots for this pod.
func GetPodSnapshotAnnotations(obj metav1.Object) map[string]string {
	var res map[string]string

	for k, v := range obj.GetAnnotations() {
		if strings.HasPrefix(k, podAnnotationPrefix) {
			if res == nil {
				res = make(map[string]string)
			}

			res[k[len(podAnnotationPrefix):]] = v
		}
	}

	return res
}

// SetPodSnapshotAnnotation adds an annotation to a pod to indicate that
// the specified volume has a restic snapshot with the provided id.
func SetPodSnapshotAnnotation(obj metav1.Object, volumeName, snapshotID string) {
	annotations := obj.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[podAnnotationPrefix+volumeName] = snapshotID

	obj.SetAnnotations(annotations)
}

// GetVolumesToBackup returns a list of volume names to backup for
// the provided pod.
func GetVolumesToBackup(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	backupsValue := annotations[volumesToBackupAnnotation]
	if backupsValue == "" {
		return nil
	}

	return strings.Split(backupsValue, ",")
}

// SnapshotIdentifier uniquely identifies a restic snapshot
// taken by Ark.
type SnapshotIdentifier struct {
	// Repo is the name of the restic repository where the
	// snapshot is located
	Repo string

	// SnapshotID is the short ID of the restic snapshot
	SnapshotID string
}

// GetSnapshotsInBackup returns a list of all restic snapshot ids associated with
// a given Ark backup.
func GetSnapshotsInBackup(backup *arkv1api.Backup, podVolumeBackupLister arkv1listers.PodVolumeBackupLister) ([]SnapshotIdentifier, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", arkv1api.BackupNameLabel, backup.Name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

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
			Repo:       item.Spec.Pod.Namespace,
			SnapshotID: item.Status.SnapshotID,
		})
	}

	return res, nil
}

// TempCredentialsFile creates a temp file containing a restic
// encryption key for the given repo and returns its path. The
// caller should generally call os.Remove() to remove the file
// when done with it.
func TempCredentialsFile(secretLister corev1listers.SecretLister, arkNamespace, repoName string, fs filesystem.Interface) (string, error) {
	secretGetter := NewListerSecretGetter(secretLister)

	// For now, all restic repos share the same key so we don't need the repoName to fetch it.
	// When we move to full-backup encryption, we'll likely have a separate key per restic repo
	// (all within the Ark server's namespace) so GetRepositoryKey will need to take the repo
	// name as an argument as well.
	repoKey, err := GetRepositoryKey(secretGetter, arkNamespace)
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

// NewPodVolumeBackupListOptions creates a ListOptions with a label selector configured to
// find PodVolumeBackups for the backup identified by name and uid.
func NewPodVolumeBackupListOptions(name, uid string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", arkv1api.BackupNameLabel, name, arkv1api.BackupUIDLabel, uid),
	}
}

// NewPodVolumeRestoreListOptions creates a ListOptions with a label selector configured to
// find PodVolumeRestores for the restore identified by name and uid.
func NewPodVolumeRestoreListOptions(name, uid string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", arkv1api.RestoreNameLabel, name, arkv1api.RestoreUIDLabel, uid),
	}
}
