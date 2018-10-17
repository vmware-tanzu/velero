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

package persistence

import (
	"fmt"
	"path"
	"strings"
)

// ObjectStoreLayout defines how Ark's persisted files map to
// keys in an object storage bucket.
type ObjectStoreLayout struct {
	rootPrefix string
	subdirs    map[string]string
}

func NewObjectStoreLayout(prefix string) *ObjectStoreLayout {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	subdirs := map[string]string{
		"backups":  path.Join(prefix, "backups") + "/",
		"restores": path.Join(prefix, "restores") + "/",
		"restic":   path.Join(prefix, "restic") + "/",
		"metadata": path.Join(prefix, "metadata") + "/",
	}

	return &ObjectStoreLayout{
		rootPrefix: prefix,
		subdirs:    subdirs,
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

func (l *ObjectStoreLayout) getRevisionKey() string {
	return path.Join(l.subdirs["metadata"], "revision")
}

func (l *ObjectStoreLayout) getBackupDir(backup string) string {
	return path.Join(l.subdirs["backups"], backup) + "/"
}

func (l *ObjectStoreLayout) getRestoreDir(restore string) string {
	return path.Join(l.subdirs["restores"], restore) + "/"
}

func (l *ObjectStoreLayout) getBackupMetadataKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, "ark-backup.json")
}

func (l *ObjectStoreLayout) getBackupContentsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s.tar.gz", backup))
}

func (l *ObjectStoreLayout) getBackupLogKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-logs.gz", backup))
}

func (l *ObjectStoreLayout) getBackupVolumeSnapshotsKey(backup string) string {
	return path.Join(l.subdirs["backups"], backup, fmt.Sprintf("%s-volumesnapshots.json.gz", backup))
}

func (l *ObjectStoreLayout) getRestoreLogKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-logs.gz", restore))
}

func (l *ObjectStoreLayout) getRestoreResultsKey(restore string) string {
	return path.Join(l.subdirs["restores"], restore, fmt.Sprintf("restore-%s-results.gz", restore))
}
