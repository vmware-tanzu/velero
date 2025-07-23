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

package persistence

import (
	"fmt"
	"path"
	"strings"
)

// ObjectStoreLayout defines how Velero's persisted files map to
// keys in an object storage bucket.
type ObjectStoreLayout struct {
	rootPrefix     string
	filenameSuffix string
	subdirs        map[string]string
}

func NewObjectStoreLayout(prefix, filenameSuffix string) *ObjectStoreLayout {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	subdirs := map[string]string{
		"backups":  path.Join(prefix, "backups") + "/",
		"restores": path.Join(prefix, "restores") + "/",
		"restic":   path.Join(prefix, "restic") + "/",
		"metadata": path.Join(prefix, "metadata") + "/",
		"plugins":  path.Join(prefix, "plugins") + "/",
		"kopia":    path.Join(prefix, "kopia") + "/",
	}

	return &ObjectStoreLayout{
		rootPrefix:     prefix,
		filenameSuffix: filenameSuffix,
		subdirs:        subdirs,
	}
}

// GetResticDir returns the full prefix representing the restic
// directory within an object storage bucket containing a backup
// store.
func (l *ObjectStoreLayout) GetResticDir() string {
	return l.subdirs["restic"]
}

func (l *ObjectStoreLayout) isValidSubdir(name string) bool {
	_, ok := l.subdirs[name]
	return ok
}

func (l *ObjectStoreLayout) getBackupDir(backup string) string {
	return path.Join(l.subdirs["backups"], backup) + "/"
}

func (l *ObjectStoreLayout) getRestoreDir(restore string) string {
	return path.Join(l.subdirs["restores"], restore) + "/"
}

func (l *ObjectStoreLayout) getBackupMetadataKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("velero-backup.json%s", l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupContentsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s.tar.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupLogKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-logs.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getPodVolumeBackupsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-podvolumebackups.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupVolumeSnapshotsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-volumesnapshots.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupItemOperationsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-itemoperations.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupResourceListKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-resource-list.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getRestoreLogKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-logs.gz%s", restore, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getRestoreResultsKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-results.gz%s", restore, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getRestoreResourceListKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-resource-list.json.gz%s", restore, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getRestoreItemOperationsKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-itemoperations.json.gz%s", restore, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getCSIVolumeSnapshotKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-csi-volumesnapshots.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getCSIVolumeSnapshotContentsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-csi-volumesnapshotcontents.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getCSIVolumeSnapshotClassesKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-csi-volumesnapshotclasses.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupResultsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-results.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getBackupVolumeInfoKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-volumeinfo.json.gz%s", backup, l.filenameSuffix))
}

func (l *ObjectStoreLayout) getRestoreVolumeInfoKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("%s-volumeinfo.json.gz%s", restore, l.filenameSuffix))
}
