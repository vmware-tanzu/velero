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
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/scheme"
	"github.com/heptio/ark/pkg/volume"
)

// BackupStore defines operations for creating, retrieving, and deleting
// Ark backup and restore data in/from a persistent backup store.
type BackupStore interface {
	IsValid() error
	GetRevision() (string, error)

	ListBackups() ([]string, error)

	PutBackup(name string, metadata, contents, log, volumeSnapshots io.Reader) error
	GetBackupMetadata(name string) (*arkv1api.Backup, error)
	GetBackupVolumeSnapshots(name string) ([]*volume.Snapshot, error)
	GetBackupContents(name string) (io.ReadCloser, error)
	DeleteBackup(name string) error

	PutRestoreLog(backup, restore string, log io.Reader) error
	PutRestoreResults(backup, restore string, results io.Reader) error
	DeleteRestore(name string) error

	GetDownloadURL(target arkv1api.DownloadTarget) (string, error)
}

// DownloadURLTTL is how long a download URL is valid for.
const DownloadURLTTL = 10 * time.Minute

type objectBackupStore struct {
	objectStore cloudprovider.ObjectStore
	bucket      string
	layout      *ObjectStoreLayout
	logger      logrus.FieldLogger
}

// ObjectStoreGetter is a type that can get a cloudprovider.ObjectStore
// from a provider name.
type ObjectStoreGetter interface {
	GetObjectStore(provider string) (cloudprovider.ObjectStore, error)
}

func NewObjectBackupStore(location *arkv1api.BackupStorageLocation, objectStoreGetter ObjectStoreGetter, logger logrus.FieldLogger) (BackupStore, error) {
	if location.Spec.ObjectStorage == nil {
		return nil, errors.New("backup storage location does not use object storage")
	}

	if location.Spec.Provider == "" {
		return nil, errors.New("object storage provider name must not be empty")
	}

	objectStore, err := objectStoreGetter.GetObjectStore(location.Spec.Provider)
	if err != nil {
		return nil, err
	}

	// add the bucket name to the config map so that object stores can use
	// it when initializing. The AWS object store uses this to determine the
	// bucket's region when setting up its client.
	if location.Spec.ObjectStorage != nil {
		if location.Spec.Config == nil {
			location.Spec.Config = make(map[string]string)
		}
		location.Spec.Config["bucket"] = location.Spec.ObjectStorage.Bucket
	}

	if err := objectStore.Init(location.Spec.Config); err != nil {
		return nil, err
	}

	log := logger.WithFields(logrus.Fields(map[string]interface{}{
		"bucket": location.Spec.ObjectStorage.Bucket,
		"prefix": location.Spec.ObjectStorage.Prefix,
	}))

	return &objectBackupStore{
		objectStore: objectStore,
		bucket:      location.Spec.ObjectStorage.Bucket,
		layout:      NewObjectStoreLayout(location.Spec.ObjectStorage.Prefix),
		logger:      log,
	}, nil
}

func (s *objectBackupStore) IsValid() error {
	dirs, err := s.objectStore.ListCommonPrefixes(s.bucket, s.layout.rootPrefix, "/")
	if err != nil {
		return errors.WithStack(err)
	}

	var invalid []string
	for _, dir := range dirs {
		subdir := strings.TrimSuffix(strings.TrimPrefix(dir, s.layout.rootPrefix), "/")
		if !s.layout.isValidSubdir(subdir) {
			invalid = append(invalid, subdir)
		}
	}

	if len(invalid) > 0 {
		// don't include more than 3 invalid dirs in the error message
		if len(invalid) > 3 {
			return errors.Errorf("Backup store contains %d invalid top-level directories: %v", len(invalid), append(invalid[:3], "..."))
		}
		return errors.Errorf("Backup store contains invalid top-level directories: %v", invalid)
	}

	return nil
}

func (s *objectBackupStore) ListBackups() ([]string, error) {
	prefixes, err := s.objectStore.ListCommonPrefixes(s.bucket, s.layout.subdirs["backups"], "/")
	if err != nil {
		return nil, err
	}
	if len(prefixes) == 0 {
		return []string{}, nil
	}

	output := make([]string, 0, len(prefixes))

	for _, prefix := range prefixes {
		// values returned from a call to cloudprovider.ObjectStore's
		// ListCommonPrefixes method return the *full* prefix, inclusive
		// of s.backupsPrefix, and include the delimiter ("/") as a suffix. Trim
		// each of those off to get the backup name.
		backupName := strings.TrimSuffix(strings.TrimPrefix(prefix, s.layout.subdirs["backups"]), "/")

		output = append(output, backupName)
	}

	return output, nil
}

func (s *objectBackupStore) PutBackup(name string, metadata, contents, log, volumeSnapshots io.Reader) error {
	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupLogKey(name), log); err != nil {
		// Uploading the log file is best-effort; if it fails, we log the error but it doesn't impact the
		// backup's status.
		s.logger.WithError(err).WithField("backup", name).Error("Error uploading log file")
	}

	if metadata == nil {
		// If we don't have metadata, something failed, and there's no point in continuing. An object
		// storage bucket that is missing the metadata file can't be restored, nor can its logs be
		// viewed.
		return nil
	}

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupMetadataKey(name), metadata); err != nil {
		// failure to upload metadata file is a hard-stop
		return err
	}

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupContentsKey(name), contents); err != nil {
		deleteErr := s.objectStore.DeleteObject(s.bucket, s.layout.getBackupMetadataKey(name))
		return kerrors.NewAggregate([]error{err, deleteErr})
	}

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupVolumeSnapshotsKey(name), volumeSnapshots); err != nil {
		errs := []error{err}

		deleteErr := s.objectStore.DeleteObject(s.bucket, s.layout.getBackupContentsKey(name))
		errs = append(errs, deleteErr)

		deleteErr = s.objectStore.DeleteObject(s.bucket, s.layout.getBackupMetadataKey(name))
		errs = append(errs, deleteErr)

		return kerrors.NewAggregate(errs)
	}

	if err := s.putRevision(); err != nil {
		s.logger.WithField("backup", name).WithError(err).Warn("Error updating backup store revision")
	}

	return nil
}

func (s *objectBackupStore) GetBackupMetadata(name string) (*arkv1api.Backup, error) {
	key := s.layout.getBackupMetadataKey(name)

	res, err := s.objectStore.GetObject(s.bucket, key)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := ioutil.ReadAll(res)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decoder := scheme.Codecs.UniversalDecoder(arkv1api.SchemeGroupVersion)
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backupObj, ok := obj.(*arkv1api.Backup)
	if !ok {
		return nil, errors.Errorf("unexpected type for %s/%s: %T", s.bucket, key, obj)
	}

	return backupObj, nil
}

func keyExists(objectStore cloudprovider.ObjectStore, bucket, prefix, key string) (bool, error) {
	keys, err := objectStore.ListObjects(bucket, prefix)
	if err != nil {
		return false, err
	}

	var found bool
	for _, existing := range keys {
		if key == existing {
			found = true
			break
		}
	}

	return found, nil
}

func (s *objectBackupStore) GetBackupVolumeSnapshots(name string) ([]*volume.Snapshot, error) {
	key := s.layout.getBackupVolumeSnapshotsKey(name)

	// if the volumesnapshots file doesn't exist, we don't want to return an error, since
	// a legacy backup or a backup with no snapshots would not have this file, so check for
	// its existence before attempting to get its contents.
	ok, err := keyExists(s.objectStore, s.bucket, s.layout.getBackupDir(name), key)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}

	res, err := s.objectStore.GetObject(s.bucket, key)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	gzr, err := gzip.NewReader(res)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer gzr.Close()

	var volumeSnapshots []*volume.Snapshot
	if err := json.NewDecoder(gzr).Decode(&volumeSnapshots); err != nil {
		return nil, errors.Wrap(err, "error decoding object data")
	}

	return volumeSnapshots, nil
}

func (s *objectBackupStore) GetBackupContents(name string) (io.ReadCloser, error) {
	return s.objectStore.GetObject(s.bucket, s.layout.getBackupContentsKey(name))
}

func (s *objectBackupStore) DeleteBackup(name string) error {
	objects, err := s.objectStore.ListObjects(s.bucket, s.layout.getBackupDir(name))
	if err != nil {
		return err
	}

	var errs []error
	for _, key := range objects {
		s.logger.WithFields(logrus.Fields{
			"key": key,
		}).Debug("Trying to delete object")
		if err := s.objectStore.DeleteObject(s.bucket, key); err != nil {
			errs = append(errs, err)
		}
	}

	if err := s.putRevision(); err != nil {
		s.logger.WithField("backup", name).WithError(err).Warn("Error updating backup store revision")
	}

	return errors.WithStack(kerrors.NewAggregate(errs))
}

func (s *objectBackupStore) DeleteRestore(name string) error {
	objects, err := s.objectStore.ListObjects(s.bucket, s.layout.getRestoreDir(name))
	if err != nil {
		return err
	}

	var errs []error
	for _, key := range objects {
		s.logger.WithFields(logrus.Fields{
			"key": key,
		}).Debug("Trying to delete object")
		if err := s.objectStore.DeleteObject(s.bucket, key); err != nil {
			errs = append(errs, err)
		}
	}

	if err = s.putRevision(); err != nil {
		errs = append(errs, err)
	}

	return errors.WithStack(kerrors.NewAggregate(errs))
}

func (s *objectBackupStore) PutRestoreLog(backup string, restore string, log io.Reader) error {
	return s.objectStore.PutObject(s.bucket, s.layout.getRestoreLogKey(restore), log)
}

func (s *objectBackupStore) PutRestoreResults(backup string, restore string, results io.Reader) error {
	return s.objectStore.PutObject(s.bucket, s.layout.getRestoreResultsKey(restore), results)
}

func (s *objectBackupStore) GetDownloadURL(target arkv1api.DownloadTarget) (string, error) {
	switch target.Kind {
	case arkv1api.DownloadTargetKindBackupContents:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupContentsKey(target.Name), DownloadURLTTL)
	case arkv1api.DownloadTargetKindBackupLog:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupLogKey(target.Name), DownloadURLTTL)
	case arkv1api.DownloadTargetKindBackupVolumeSnapshots:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupVolumeSnapshotsKey(target.Name), DownloadURLTTL)
	case arkv1api.DownloadTargetKindRestoreLog:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getRestoreLogKey(target.Name), DownloadURLTTL)
	case arkv1api.DownloadTargetKindRestoreResults:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getRestoreResultsKey(target.Name), DownloadURLTTL)
	default:
		return "", errors.Errorf("unsupported download target kind %q", target.Kind)
	}
}

func (s *objectBackupStore) GetRevision() (string, error) {
	rdr, err := s.objectStore.GetObject(s.bucket, s.layout.getRevisionKey())
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadAll(rdr)
	if err != nil {
		return "", errors.Wrap(err, "error reading contents of revision file")
	}

	return string(bytes), nil
}

func (s *objectBackupStore) putRevision() error {
	rdr := strings.NewReader(uuid.NewV4().String())

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getRevisionKey(), rdr); err != nil {
		return errors.Wrap(err, "error updating revision file")
	}

	return nil
}

func seekToBeginning(r io.Reader) error {
	seeker, ok := r.(io.Seeker)
	if !ok {
		return nil
	}

	_, err := seeker.Seek(0, 0)
	return err
}

func seekAndPutObject(objectStore cloudprovider.ObjectStore, bucket, key string, file io.Reader) error {
	if file == nil {
		return nil
	}

	if err := seekToBeginning(file); err != nil {
		return errors.WithStack(err)
	}

	return objectStore.PutObject(bucket, key, file)
}
