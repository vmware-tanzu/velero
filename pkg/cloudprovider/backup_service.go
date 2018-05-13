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

// BackupLister knows how to list backups in object storage.
type BackupLister interface {
	// ListBackups lists all the api.Backups in object storage for the given bucket.
	ListBackups(bucket string) ([]*api.Backup, error)
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

func DeleteBackupMetadata(objectStore ObjectStore, bucket, backupName string) error {
	metadataKey := getMetadataKey(backupName)
	return objectStore.DeleteObject(bucket, metadataKey)
}

func UploadBackupData(objectStore ObjectStore, bucket, backupName string, backup io.Reader) error {
	backupKey := getBackupContentsKey(backupName, backupName)
	return seekAndPutObject(objectStore, bucket, backupKey, backup)
}

func UploadBackup(logger logrus.FieldLogger, objectStore ObjectStore, bucket, backupName string, metadata, backup, log io.Reader) error {
	if err := UploadBackupLog(objectStore, bucket, backupName, log); err != nil {
		// Uploading the log file is best-effort; if it fails, we log the error but it doesn't impact the
		// backup's status.
		logger.WithError(err).WithField("bucket", bucket).Error("Error uploading log file")
	}

	if metadata == nil {
		// If we don't have metadata, something failed, and there's no point in continuing. An object
		// storage bucket that is missing the metadata file can't be restored, nor can its logs be
		// viewed.
		return nil
	}

	// upload metadata file
	if err := UploadBackupMetadata(objectStore, bucket, backupName, metadata); err != nil {
		// failure to upload metadata file is a hard-stop
		return err
	}

	// upload tar file
	if err := UploadBackupData(objectStore, bucket, backupName, backup); err != nil {
		// try to delete the metadata file since the data upload failed
		deleteErr := DeleteBackupMetadata(objectStore, bucket, backupName)
		return kerrors.NewAggregate([]error{err, deleteErr})
	}

	return nil
}

// DownloadBackupFunc is a function that can download backup metadata from a bucket in object storage.
type DownloadBackupFunc func(objectStore ObjectStore, bucket, backupName string) (io.ReadCloser, error)

// DownloadBackup downloads an Ark backup with the specified object key from object storage via the cloud API.
// It returns the snapshot metadata and data (separately), or an error if a problem is encountered
// downloading or reading the file from the cloud API.
func DownloadBackup(objectStore ObjectStore, bucket, backupName string) (io.ReadCloser, error) {
	return objectStore.GetObject(bucket, getBackupContentsKey(backupName, backupName))
}

type liveBackupLister struct {
	logger      logrus.FieldLogger
	objectStore ObjectStore
}

func NewLiveBackupLister(logger logrus.FieldLogger, objectStore ObjectStore) BackupLister {
	return &liveBackupLister{
		logger:      logger,
		objectStore: objectStore,
	}
}

func (l *liveBackupLister) ListBackups(bucket string) ([]*api.Backup, error) {
	return ListBackups(l.logger, l.objectStore, bucket)
}

func ListBackups(logger logrus.FieldLogger, objectStore ObjectStore, bucket string) ([]*api.Backup, error) {
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

//GetBackupFunc is a function that can retrieve backup metadata from an object store
type GetBackupFunc func(objectStore ObjectStore, bucket, backupName string) (*api.Backup, error)

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

// DeleteBackupDirFunc is a function that can delete a backup directory from a bucket in object storage.
type DeleteBackupDirFunc func(logger logrus.FieldLogger, objectStore ObjectStore, bucket, backupName string) error

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

// CreateSignedURLFunc is a function that can create a signed URL for an object in object storage.
type CreateSignedURLFunc func(objectStore ObjectStore, target api.DownloadTarget, bucket, directory string, ttl time.Duration) (string, error)

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

// UploadRestoreLogFunc is a function that can upload a restore log to a bucket in object storage.
type UploadRestoreLogFunc func(objectStore ObjectStore, bucket, backup, restore string, log io.Reader) error

// UploadRestoreLog uploads the restore's log file to object storage.
func UploadRestoreLog(objectStore ObjectStore, bucket, backup, restore string, log io.Reader) error {
	key := getRestoreLogKey(backup, restore)
	return objectStore.PutObject(bucket, key, log)
}

// UploadRestoreResultsFunc is a function that can upload restore results to a bucket in object storage.
type UploadRestoreResultsFunc func(objectStore ObjectStore, bucket, backup, restore string, results io.Reader) error

// UploadRestoreResults uploads the restore's results file to object storage.
func UploadRestoreResults(objectStore ObjectStore, bucket, backup, restore string, results io.Reader) error {
	key := getRestoreResultsKey(backup, restore)
	return objectStore.PutObject(bucket, key, results)
}
