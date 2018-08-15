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

package storage

import (
	"io"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/plugin"
)

type Accessor interface {
	SaveBackupLog(log io.Reader) error

	SaveBackupMetadata(metadata io.Reader) error

	DeleteBackupMetadata() error

	SaveBackupData(backup io.Reader) error

	SaveBackup(metadata, backup, log io.Reader) error

	GetBackupData() (io.ReadCloser, error)

	GetBackupMetadata() (*arkv1api.Backup, error)

	DeleteBackup() error

	SaveRestoreLog(restore string, log io.Reader) error

	SaveRestoreResults(restore string, results io.Reader) error
}

func BackupAccessor(backup string, location *arkv1api.BackupStorageLocation, pluginManager plugin.Manager, log logrus.FieldLogger) (Accessor, error) {
	switch {
	case location.Spec.ObjectStorage != nil:
		if location.Spec.Provider == "" {
			return nil, errors.New("object storage provider name must not be empty")
		}

		objectStore, err := pluginManager.GetObjectStore(location.Spec.Provider)
		if err != nil {
			return nil, err
		}

		if err := objectStore.Init(location.Spec.Config); err != nil {
			return nil, err
		}

		return &objectStoreBackupAccessor{
			objectStore: objectStore,
			bucket:      location.Spec.ObjectStorage.Bucket,
			prefix:      location.Spec.ObjectStorage.Prefix,
			backup:      backup,
			log:         log,
		}, nil
	}

	return nil, errors.New("no backup storage location backend configured")
}
