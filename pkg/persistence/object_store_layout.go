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

	"k8s.io/apimachinery/pkg/util/sets"
)

var validRootDirs = sets.NewString("backups", "restores")

type objectStoreLayout struct {
	rootPrefix  string
	backupsDir  string
	restoresDir string
}

func newObjectStoreLayout(prefix string) *objectStoreLayout {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	backupsDir := path.Join(prefix, "backups") + "/"
	restoresDir := path.Join(prefix, "restores") + "/"

	return &objectStoreLayout{
		rootPrefix:  prefix,
		backupsDir:  backupsDir,
		restoresDir: restoresDir,
	}
}

func (l *objectStoreLayout) getBackupDir(backup string) string {
	return path.Join(l.backupsDir, backup) + "/"
}

func (l *objectStoreLayout) getRestoreDir(restore string) string {
	return path.Join(l.restoresDir, restore) + "/"
}

func (l *objectStoreLayout) getBackupMetadataKey(backup string) string {
	return path.Join(l.backupsDir, backup, "ark-backup.json")
}

func (l *objectStoreLayout) getBackupContentsKey(backup string) string {
	return path.Join(l.backupsDir, backup, fmt.Sprintf("%s.tar.gz", backup))
}

func (l *objectStoreLayout) getBackupLogKey(backup string) string {
	return path.Join(l.backupsDir, backup, fmt.Sprintf("%s-logs.gz", backup))
}

func (l *objectStoreLayout) getRestoreLogKey(restore string) string {
	return path.Join(l.restoresDir, restore, fmt.Sprintf("restore-%s-logs.gz", restore))
}

func (l *objectStoreLayout) getRestoreResultsKey(restore string) string {
	return path.Join(l.restoresDir, restore, fmt.Sprintf("restore-%s-results.gz", restore))
}
