/*
Copyright 2017 the Heptio Ark contributors.

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
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	kerrors "k8s.io/apimachinery/pkg/util/errors"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/scheme"
)

// BackupGetter knows how to list backups in object storage.
type BackupGetter interface {
	// GetAllBackups lists all the api.Backups in object storage for the given bucket.
	GetAllBackups(bucket string) ([]*api.Backup, error)
}

const (
	metadataFileFormatString       = "%s/ark-backup.json"
	backupFileFormatString         = "%s/%s.tar.gz"
	backupLogFileFormatString      = "%s/%s-logs.gz"
	restoreLogFileFormatString     = "%s/restore-%s-logs.gz"
	restoreResultsFileFormatString = "%s/restore-%s-results.gz"
)

func getMetadataKey(directory string) string {
	return fmt.Sprintf(metadataFileFormatString, directory)
}

func getBackupContentsKey(directory, backup string) string {
	return fmt.Sprintf(backupFileFormatString, directory, backup)
}

func getBackupLogKey(directory, backup string) string {
	return fmt.Sprintf(backupLogFileFormatString, directory, backup)
}

func getRestoreLogKey(directory, restore string) string {
	return fmt.Sprintf(restoreLogFileFormatString, directory, restore)
}

func getRestoreResultsKey(directory, restore string) string {
	return fmt.Sprintf(restoreResultsFileFormatString, directory, restore)
}

func seekToBeginning(r io.Reader) error {
	seeker, ok := r.(io.Seeker)
	if !ok {
		return nil
	}

	_, err := seeker.Seek(0, 0)
	return err
}

func seekAndPutObject(objectStore ObjectStore, bucket, key string, file io.Reader) error {
	if file == nil {
		return nil
	}

	if err := seekToBeginning(file); err != nil {
		return errors.WithStack(err)
	}

	return objectStore.PutObject(bucket, key, file)
}

func UploadBackupLog(objectStore ObjectStore, bucket, backupName string, log io.Reader) error {
	logKey := getBackupLogKey(backupName, backupName)
	return seekAndPutObject(objectStore, bucket, logKey, log)
}

func UploadBackupMetadata(objectStore ObjectStore, bucket, backupName string, metadata io.Reader) error {
	metadataKey := getMetadataKey(backupName)
	return seekAndPutObject(objectStore, bucket, metadataKey, metadata)
}

func UploadBackup(objectStore ObjectStore, bucket, backupName string, backup io.Reader) error {
	if err := seekAndPutObject(objectStore, bucket, getBackupContentsKey(backupName, backupName), backup); err != nil {
		// try to delete the metadata file since the data upload failed
		metadataKey := getMetadataKey(backupName)
		deleteErr := objectStore.DeleteObject(bucket, metadataKey)

		return kerrors.NewAggregate([]error{err, deleteErr})
	}

	return nil
}

// DownloadBackup downloads an Ark backup with the specified object key from object storage via the cloud API.
// It returns the snapshot metadata and data (separately), or an error if a problem is encountered
// downloading or reading the file from the cloud API.
func DownloadBackup(objectStore ObjectStore, bucket, backupName string) (io.ReadCloser, error) {
	return objectStore.GetObject(bucket, getBackupContentsKey(backupName, backupName))
}

func GetAllBackups(logger logrus.FieldLogger, objectStore ObjectStore, bucket string) ([]*api.Backup, error) {
	prefixes, err := objectStore.ListCommonPrefixes(bucket, "/")
	if err != nil {
		return nil, err
	}
	if len(prefixes) == 0 {
		return []*api.Backup{}, nil
	}

	output := make([]*api.Backup, 0, len(prefixes))

	for _, backupDir := range prefixes {
		backup, err := GetBackup(objectStore, bucket, backupDir)
		if err != nil {
			logger.WithError(err).WithField("dir", backupDir).Error("Error reading backup directory")
			continue
		}

		output = append(output, backup)
	}

	return output, nil
}

// GetBackup gets the specified api.Backup from the given bucket in object storage.
func GetBackup(objectStore ObjectStore, bucket, backupName string) (*api.Backup, error) {
	key := getMetadataKey(backupName)

	res, err := objectStore.GetObject(bucket, key)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := ioutil.ReadAll(res)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decoder := scheme.Codecs.UniversalDecoder(api.SchemeGroupVersion)
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backup, ok := obj.(*api.Backup)
	if !ok {
		return nil, errors.Errorf("unexpected type for %s/%s: %T", bucket, key, obj)
	}

	return backup, nil
}

// DeleteBackupDir deletes all files in object storage for the given backup.
func DeleteBackupDir(logger logrus.FieldLogger, objectStore ObjectStore, bucket, backupName string) error {
	objects, err := objectStore.ListObjects(bucket, backupName+"/")
	if err != nil {
		return err
	}

	var errs []error
	for _, key := range objects {
		logger.WithFields(logrus.Fields{
			"bucket": bucket,
			"key":    key,
		}).Debug("Trying to delete object")
		if err := objectStore.DeleteObject(bucket, key); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.WithStack(kerrors.NewAggregate(errs))
}

// CreateSignedURL creates a pre-signed URL that can be used to download a file from object
// storage. The URL expires after ttl.
func CreateSignedURL(objectStore ObjectStore, target api.DownloadTarget, bucket, directory string, ttl time.Duration) (string, error) {
	switch target.Kind {
	case api.DownloadTargetKindBackupContents:
		return objectStore.CreateSignedURL(bucket, getBackupContentsKey(directory, target.Name), ttl)
	case api.DownloadTargetKindBackupLog:
		return objectStore.CreateSignedURL(bucket, getBackupLogKey(directory, target.Name), ttl)
	case api.DownloadTargetKindRestoreLog:
		return objectStore.CreateSignedURL(bucket, getRestoreLogKey(directory, target.Name), ttl)
	case api.DownloadTargetKindRestoreResults:
		return objectStore.CreateSignedURL(bucket, getRestoreResultsKey(directory, target.Name), ttl)
	default:
		return "", errors.Errorf("unsupported download target kind %q", target.Kind)
	}
}

// UploadRestoreLog uploads the restore's log file to object storage.
func UploadRestoreLog(objectStore ObjectStore, bucket, backup, restore string, log io.Reader) error {
	key := getRestoreLogKey(backup, restore)
	return objectStore.PutObject(bucket, key, log)
}

// UploadRestoreResults uploads the restore's results file to object storage.
func UploadRestoreResults(objectStore ObjectStore, bucket, backup, restore string, results io.Reader) error {
	key := getRestoreResultsKey(backup, restore)
	return objectStore.PutObject(bucket, key, results)
}

// cachedBackupService wraps a real backup service with a cache for getting cloud backups.
// type cachedBackupService struct {
// 	BackupService
// 	cache BackupGetter
// }

// // NewBackupServiceWithCachedBackupGetter returns a BackupService that uses a cache for
// // GetAllBackups().
// func NewBackupServiceWithCachedBackupGetter(
// 	ctx context.Context,
// 	delegate BackupService,
// 	resyncPeriod time.Duration,
// 	logger logrus.FieldLogger,
// ) BackupService {
// 	return &cachedBackupService{
// 		BackupService: delegate,
// 		cache:         NewBackupCache(ctx, delegate, resyncPeriod, logger),
// 	}
// }

// func (c *cachedBackupService) GetAllBackups(bucketName string) ([]*api.Backup, error) {
// 	return c.cache.GetAllBackups(bucketName)
// }
