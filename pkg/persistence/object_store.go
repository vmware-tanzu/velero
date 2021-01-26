/*
Copyright 2018, 2020 the Velero contributors.

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

	snapshotv1beta1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type BackupInfo struct {
	Name string
	Metadata,
	Contents,
	Log,
	PodVolumeBackups,
	VolumeSnapshots,
	BackupResourceList,
	CSIVolumeSnapshots,
	CSIVolumeSnapshotContents io.Reader
}

// BackupStore defines operations for creating, retrieving, and deleting
// Velero backup and restore data in/from a persistent backup store.
type BackupStore interface {
	IsValid() error

	ListBackups() ([]string, error)

	PutBackup(info BackupInfo) error
	GetBackupMetadata(name string) (*velerov1api.Backup, error)
	GetBackupVolumeSnapshots(name string) ([]*volume.Snapshot, error)
	GetPodVolumeBackups(name string) ([]*velerov1api.PodVolumeBackup, error)
	GetBackupContents(name string) (io.ReadCloser, error)
	GetCSIVolumeSnapshots(name string) ([]*snapshotv1beta1api.VolumeSnapshot, error)
	GetCSIVolumeSnapshotContents(name string) ([]*snapshotv1beta1api.VolumeSnapshotContent, error)

	// BackupExists checks if the backup metadata file exists in object storage.
	BackupExists(bucket, backupName string) (bool, error)

	DeleteBackup(name string) error

	PutRestoreLog(backup, restore string, log io.Reader) error
	PutRestoreResults(backup, restore string, results io.Reader) error
	DeleteRestore(name string) error

	GetDownloadURL(target velerov1api.DownloadTarget) (string, error)
}

// DownloadURLTTL is how long a download URL is valid for.
const DownloadURLTTL = 10 * time.Minute

type objectBackupStore struct {
	objectStore velero.ObjectStore
	bucket      string
	layout      *ObjectStoreLayout
	logger      logrus.FieldLogger
}

// ObjectStoreGetter is a type that can get a velero.ObjectStore
// from a provider name.
type ObjectStoreGetter interface {
	GetObjectStore(provider string) (velero.ObjectStore, error)
}

func NewObjectBackupStore(location *velerov1api.BackupStorageLocation, objectStoreGetter ObjectStoreGetter, logger logrus.FieldLogger) (BackupStore, error) {
	if location.Spec.ObjectStorage == nil {
		return nil, errors.New("backup storage location does not use object storage")
	}

	if location.Spec.Provider == "" {
		return nil, errors.New("object storage provider name must not be empty")
	}

	// trim off any leading/trailing slashes
	bucket := strings.Trim(location.Spec.ObjectStorage.Bucket, "/")
	prefix := strings.Trim(location.Spec.ObjectStorage.Prefix, "/")

	// if there are any slashes in the middle of 'bucket', the user
	// probably put <bucket>/<prefix> in the bucket field, which we
	// don't support.
	if strings.Contains(bucket, "/") {
		return nil, errors.Errorf("backup storage location's bucket name %q must not contain a '/' (if using a prefix, put it in the 'Prefix' field instead)", location.Spec.ObjectStorage.Bucket)
	}

	// add the bucket name and prefix to the config map so that object stores
	// can use them when initializing. The AWS object store uses the bucket
	// name to determine the bucket's region when setting up its client.
	if location.Spec.ObjectStorage != nil {
		if location.Spec.Config == nil {
			location.Spec.Config = make(map[string]string)
		}
		location.Spec.Config["bucket"] = bucket
		location.Spec.Config["prefix"] = prefix
		// Only include a CACert if it's specified in order to maintain compatibility with plugins that don't expect it.
		if location.Spec.ObjectStorage.CACert != nil {
			location.Spec.Config["caCert"] = string(location.Spec.ObjectStorage.CACert)
		}
	}

	objectStore, err := objectStoreGetter.GetObjectStore(location.Spec.Provider)
	if err != nil {
		return nil, err
	}

	if err := objectStore.Init(location.Spec.Config); err != nil {
		return nil, err
	}

	log := logger.WithFields(logrus.Fields(map[string]interface{}{
		"bucket": bucket,
		"prefix": prefix,
	}))

	return &objectBackupStore{
		objectStore: objectStore,
		bucket:      bucket,
		layout:      NewObjectStoreLayout(prefix),
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
		// values returned from a call to ObjectStore's
		// ListCommonPrefixes method return the *full* prefix, inclusive
		// of s.backupsPrefix, and include the delimiter ("/") as a suffix. Trim
		// each of those off to get the backup name.
		backupName := strings.TrimSuffix(strings.TrimPrefix(prefix, s.layout.subdirs["backups"]), "/")

		output = append(output, backupName)
	}

	return output, nil
}

func (s *objectBackupStore) PutBackup(info BackupInfo) error {
	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupLogKey(info.Name), info.Log); err != nil {
		// Uploading the log file is best-effort; if it fails, we log the error but it doesn't impact the
		// backup's status.
		s.logger.WithError(err).WithField("backup", info.Name).Error("Error uploading log file")
	}

	if info.Metadata == nil {
		// If we don't have metadata, something failed, and there's no point in continuing. An object
		// storage bucket that is missing the metadata file can't be restored, nor can its logs be
		// viewed.
		return nil
	}

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupMetadataKey(info.Name), info.Metadata); err != nil {
		// failure to upload metadata file is a hard-stop
		return err
	}

	if err := seekAndPutObject(s.objectStore, s.bucket, s.layout.getBackupContentsKey(info.Name), info.Contents); err != nil {
		deleteErr := s.objectStore.DeleteObject(s.bucket, s.layout.getBackupMetadataKey(info.Name))
		return kerrors.NewAggregate([]error{err, deleteErr})
	}

	// Since the logic for all of these files is the exact same except for the name and the contents,
	// use a map literal to iterate through them and write them to the bucket.
	var backupObjs = map[string]io.Reader{
		s.layout.getPodVolumeBackupsKey(info.Name):          info.PodVolumeBackups,
		s.layout.getBackupVolumeSnapshotsKey(info.Name):     info.VolumeSnapshots,
		s.layout.getBackupResourceListKey(info.Name):        info.BackupResourceList,
		s.layout.getCSIVolumeSnapshotKey(info.Name):         info.CSIVolumeSnapshots,
		s.layout.getCSIVolumeSnapshotContentsKey(info.Name): info.CSIVolumeSnapshotContents,
	}

	for key, reader := range backupObjs {
		if err := seekAndPutObject(s.objectStore, s.bucket, key, reader); err != nil {
			errs := []error{err}

			// attempt to clean up the backup contents and metadata if we fail to upload and of the extra files.
			deleteErr := s.objectStore.DeleteObject(s.bucket, s.layout.getBackupContentsKey(info.Name))
			errs = append(errs, deleteErr)

			deleteErr = s.objectStore.DeleteObject(s.bucket, s.layout.getBackupMetadataKey(info.Name))
			errs = append(errs, deleteErr)
			return kerrors.NewAggregate(errs)
		}
	}

	return nil
}

func (s *objectBackupStore) GetBackupMetadata(name string) (*velerov1api.Backup, error) {
	metadataKey := s.layout.getBackupMetadataKey(name)

	res, err := s.objectStore.GetObject(s.bucket, metadataKey)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := ioutil.ReadAll(res)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decoder := scheme.Codecs.UniversalDecoder(velerov1api.SchemeGroupVersion)
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backupObj, ok := obj.(*velerov1api.Backup)
	if !ok {
		return nil, errors.Errorf("unexpected type for %s/%s: %T", s.bucket, metadataKey, obj)
	}

	return backupObj, nil
}

func (s *objectBackupStore) GetBackupVolumeSnapshots(name string) ([]*volume.Snapshot, error) {
	// if the volumesnapshots file doesn't exist, we don't want to return an error, since
	// a legacy backup or a backup with no snapshots would not have this file, so check for
	// its existence before attempting to get its contents.
	res, err := tryGet(s.objectStore, s.bucket, s.layout.getBackupVolumeSnapshotsKey(name))
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	defer res.Close()

	var volumeSnapshots []*volume.Snapshot
	if err := decode(res, &volumeSnapshots); err != nil {
		return nil, err
	}

	return volumeSnapshots, nil
}

// tryGet returns the object with the given key if it exists, nil if it does not exist,
// or an error if it was unable to check existence or get the object.
func tryGet(objectStore velero.ObjectStore, bucket, key string) (io.ReadCloser, error) {
	exists, err := objectStore.ObjectExists(bucket, key)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !exists {
		return nil, nil
	}

	return objectStore.GetObject(bucket, key)
}

// decode extracts a .json.gz file reader into the object pointed to
// by 'into'.
func decode(jsongzReader io.Reader, into interface{}) error {
	gzr, err := gzip.NewReader(jsongzReader)
	if err != nil {
		return errors.WithStack(err)
	}
	defer gzr.Close()

	if err := json.NewDecoder(gzr).Decode(into); err != nil {
		return errors.Wrap(err, "error decoding object data")
	}

	return nil
}

func (s *objectBackupStore) GetCSIVolumeSnapshots(name string) ([]*snapshotv1beta1api.VolumeSnapshot, error) {
	res, err := tryGet(s.objectStore, s.bucket, s.layout.getCSIVolumeSnapshotKey(name))
	if err != nil {
		return nil, err
	}
	if res == nil {
		// this indicates that the no CSI volumesnapshots were prensent in the backup
		return nil, nil
	}
	defer res.Close()

	var csiSnaps []*snapshotv1beta1api.VolumeSnapshot
	if err := decode(res, &csiSnaps); err != nil {
		return nil, err
	}
	return csiSnaps, nil
}

func (s *objectBackupStore) GetCSIVolumeSnapshotContents(name string) ([]*snapshotv1beta1api.VolumeSnapshotContent, error) {
	res, err := tryGet(s.objectStore, s.bucket, s.layout.getCSIVolumeSnapshotContentsKey(name))
	if err != nil {
		return nil, err
	}
	if res == nil {
		// this indicates that the no CSI volumesnapshotcontents were prensent in the backup
		return nil, nil
	}
	defer res.Close()

	var snapConts []*snapshotv1beta1api.VolumeSnapshotContent
	if err := decode(res, &snapConts); err != nil {
		return nil, err
	}
	return snapConts, nil
}

func (s *objectBackupStore) GetPodVolumeBackups(name string) ([]*velerov1api.PodVolumeBackup, error) {
	// if the podvolumebackups file doesn't exist, we don't want to return an error, since
	// a legacy backup or a backup with no pod volume backups would not have this file, so
	// check for its existence before attempting to get its contents.
	res, err := tryGet(s.objectStore, s.bucket, s.layout.getPodVolumeBackupsKey(name))
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	defer res.Close()

	var podVolumeBackups []*velerov1api.PodVolumeBackup
	if err := decode(res, &podVolumeBackups); err != nil {
		return nil, err
	}

	return podVolumeBackups, nil
}

func (s *objectBackupStore) GetBackupContents(name string) (io.ReadCloser, error) {
	return s.objectStore.GetObject(s.bucket, s.layout.getBackupContentsKey(name))
}

func (s *objectBackupStore) BackupExists(bucket, backupName string) (bool, error) {
	return s.objectStore.ObjectExists(bucket, s.layout.getBackupMetadataKey(backupName))
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

	return errors.WithStack(kerrors.NewAggregate(errs))
}

func (s *objectBackupStore) PutRestoreLog(backup string, restore string, log io.Reader) error {
	return s.objectStore.PutObject(s.bucket, s.layout.getRestoreLogKey(restore), log)
}

func (s *objectBackupStore) PutRestoreResults(backup string, restore string, results io.Reader) error {
	return s.objectStore.PutObject(s.bucket, s.layout.getRestoreResultsKey(restore), results)
}

func (s *objectBackupStore) GetDownloadURL(target velerov1api.DownloadTarget) (string, error) {
	switch target.Kind {
	case velerov1api.DownloadTargetKindBackupContents:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupContentsKey(target.Name), DownloadURLTTL)
	case velerov1api.DownloadTargetKindBackupLog:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupLogKey(target.Name), DownloadURLTTL)
	case velerov1api.DownloadTargetKindBackupVolumeSnapshots:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupVolumeSnapshotsKey(target.Name), DownloadURLTTL)
	case velerov1api.DownloadTargetKindBackupResourceList:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getBackupResourceListKey(target.Name), DownloadURLTTL)
	case velerov1api.DownloadTargetKindRestoreLog:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getRestoreLogKey(target.Name), DownloadURLTTL)
	case velerov1api.DownloadTargetKindRestoreResults:
		return s.objectStore.CreateSignedURL(s.bucket, s.layout.getRestoreResultsKey(target.Name), DownloadURLTTL)
	default:
		return "", errors.Errorf("unsupported download target kind %q", target.Kind)
	}
}

func seekToBeginning(r io.Reader) error {
	seeker, ok := r.(io.Seeker)
	if !ok {
		return nil
	}

	_, err := seeker.Seek(0, 0)
	return err
}

func seekAndPutObject(objectStore velero.ObjectStore, bucket, key string, file io.Reader) error {
	if file == nil {
		return nil
	}

	if err := seekToBeginning(file); err != nil {
		return errors.WithStack(err)
	}

	return objectStore.PutObject(bucket, key, file)
}
