/*
Copyright 2017 Heptio Inc.

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

package cloudprovider

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
)

// BackupService contains methods for working with backups in object storage.
type BackupService interface {
	BackupGetter
	// UploadBackup uploads the specified Ark backup of a set of Kubernetes API objects, whose manifests are
	// stored in the specified file, into object storage in an Ark bucket, tagged with Ark metadata. Returns
	// an error if a problem is encountered accessing the file or performing the upload via the cloud API.
	UploadBackup(bucket, name string, metadata, backup io.ReadSeeker) error

	// DownloadBackup downloads an Ark backup with the specified object key from object storage via the cloud API.
	// It returns the snapshot metadata and data (separately), or an error if a problem is encountered
	// downloading or reading the file from the cloud API.
	DownloadBackup(bucket, name string) (io.ReadCloser, error)

	// DeleteBackup deletes the backup content in object storage for the given backup.
	DeleteBackupFile(bucket, backupName string) error

	// DeleteBackup deletes the backup metadata file in object storage for the given backup.
	DeleteBackupMetadataFile(bucket, backupName string) error

	// GetBackup gets the specified api.Backup from the given bucket in object storage.
	GetBackup(bucket, name string) (*api.Backup, error)
}

// BackupGetter knows how to list backups in object storage.
type BackupGetter interface {
	// GetAllBackups lists all the api.Backups in object storage for the given bucket.
	GetAllBackups(bucket string) ([]*api.Backup, error)
}

const (
	metadataFileFormatString string = "%s/ark-backup.json"
	backupFileFormatString   string = "%s/%s.tar.gz"
)

type backupService struct {
	objectStorage ObjectStorageAdapter
	decoder       runtime.Decoder
}

var _ BackupService = &backupService{}
var _ BackupGetter = &backupService{}

// NewBackupService creates a backup service using the provided object storage adapter
func NewBackupService(objectStorage ObjectStorageAdapter) BackupService {
	return &backupService{
		objectStorage: objectStorage,
		decoder:       scheme.Codecs.UniversalDecoder(api.SchemeGroupVersion),
	}
}

func (br *backupService) UploadBackup(bucket, backupName string, metadata, backup io.ReadSeeker) error {
	// upload metadata file
	metadataKey := fmt.Sprintf(metadataFileFormatString, backupName)
	if err := br.objectStorage.PutObject(bucket, metadataKey, metadata); err != nil {
		return err
	}

	// upload tar file
	if err := br.objectStorage.PutObject(bucket, fmt.Sprintf(backupFileFormatString, backupName, backupName), backup); err != nil {
		// try to delete the metadata file since the data upload failed
		deleteErr := br.objectStorage.DeleteObject(bucket, metadataKey)

		return errors.NewAggregate([]error{err, deleteErr})
	}

	return nil
}

func (br *backupService) DownloadBackup(bucket, backupName string) (io.ReadCloser, error) {
	return br.objectStorage.GetObject(bucket, fmt.Sprintf(backupFileFormatString, backupName, backupName))
}

func (br *backupService) GetAllBackups(bucket string) ([]*api.Backup, error) {
	prefixes, err := br.objectStorage.ListCommonPrefixes(bucket, "/")
	if err != nil {
		return nil, err
	}
	if len(prefixes) == 0 {
		return []*api.Backup{}, nil
	}

	output := make([]*api.Backup, 0, len(prefixes))

	for _, backupDir := range prefixes {
		backup, err := br.GetBackup(bucket, backupDir)
		if err != nil {
			glog.Errorf("Error reading backup directory %s: %v", backupDir, err)
			continue
		}

		output = append(output, backup)
	}

	return output, nil
}

func (br *backupService) GetBackup(bucket, name string) (*api.Backup, error) {
	key := fmt.Sprintf(metadataFileFormatString, name)

	res, err := br.objectStorage.GetObject(bucket, key)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := ioutil.ReadAll(res)
	if err != nil {
		return nil, err
	}

	obj, _, err := br.decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	backup, ok := obj.(*api.Backup)
	if !ok {
		return nil, fmt.Errorf("unexpected type for %s/%s: %T", bucket, key, obj)
	}

	return backup, nil
}

func (br *backupService) DeleteBackupFile(bucket, backupName string) error {
	key := fmt.Sprintf(backupFileFormatString, backupName, backupName)
	glog.V(4).Infof("Trying to delete bucket=%s, key=%s", bucket, key)
	return br.objectStorage.DeleteObject(bucket, key)
}

func (br *backupService) DeleteBackupMetadataFile(bucket, backupName string) error {
	key := fmt.Sprintf(metadataFileFormatString, backupName)
	glog.V(4).Infof("Trying to delete bucket=%s, key=%s", bucket, key)
	return br.objectStorage.DeleteObject(bucket, key)
}

// cachedBackupService wraps a real backup service with a cache for getting cloud backups.
type cachedBackupService struct {
	BackupService
	cache BackupGetter
}

// NewBackupServiceWithCachedBackupGetter returns a BackupService that uses a cache for
// GetAllBackups().
func NewBackupServiceWithCachedBackupGetter(ctx context.Context, delegate BackupService, resyncPeriod time.Duration) BackupService {
	return &cachedBackupService{
		BackupService: delegate,
		cache:         NewBackupCache(ctx, delegate, resyncPeriod),
	}
}

func (c *cachedBackupService) GetAllBackups(bucketName string) ([]*api.Backup, error) {
	return c.cache.GetAllBackups(bucketName)
}
