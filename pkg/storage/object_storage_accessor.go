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
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/scheme"
)

type objectStoreBackupAccessor struct {
	objectStore cloudprovider.ObjectStore
	bucket      string
	prefix      string
	backup      string
	log         logrus.FieldLogger
}

var _ Accessor = &objectStoreBackupAccessor{}

func (o *objectStoreBackupAccessor) SaveBackupLog(log io.Reader) error {
	return seekAndPutObject(o.objectStore, o.bucket, getBackupLogKey(o.backup, o.backup), log)
}

func (o *objectStoreBackupAccessor) SaveBackupMetadata(backup io.Reader) error {
	return seekAndPutObject(o.objectStore, o.bucket, getMetadataKey(o.backup), backup)
}

func (o *objectStoreBackupAccessor) DeleteBackupMetadata() error {
	return o.objectStore.DeleteObject(o.bucket, getMetadataKey(o.backup))
}

func (o *objectStoreBackupAccessor) SaveBackupData(backup io.Reader) error {
	return seekAndPutObject(o.objectStore, o.bucket, getBackupContentsKey(o.backup, o.backup), backup)
}

func (o *objectStoreBackupAccessor) SaveBackup(metadata, backup, log io.Reader) error {
	if err := o.SaveBackupLog(log); err != nil {
		// Uploading the log file is best-effort; if it fails, we log the error but it doesn't impact the
		// backup's status.
		o.log.WithError(err).WithField("bucket", o.bucket).Error("Error uploading log file")
	}

	if metadata == nil {
		// If we don't have metadata, something failed, and there's no point in continuing. An object
		// storage bucket that is missing the metadata file can't be restored, nor can its logs be
		// viewed.
		return nil
	}

	// upload metadata file
	if err := o.SaveBackupMetadata(metadata); err != nil {
		// failure to upload metadata file is a hard-stop
		return err
	}

	// upload tar file
	if err := o.SaveBackupData(backup); err != nil {
		// try to delete the metadata file since the data upload failed
		deleteErr := o.DeleteBackupMetadata()
		return kerrors.NewAggregate([]error{err, deleteErr})
	}

	return nil
}

func (o *objectStoreBackupAccessor) GetBackupData() (io.ReadCloser, error) {
	return o.objectStore.GetObject(o.bucket, getBackupContentsKey(o.backup, o.backup))
}

func (o *objectStoreBackupAccessor) GetBackupMetadata() (*arkv1api.Backup, error) {
	key := getMetadataKey(o.backup)

	res, err := o.objectStore.GetObject(o.bucket, key)
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

	backup, ok := obj.(*arkv1api.Backup)
	if !ok {
		return nil, errors.Errorf("unexpected type for %s/%s: %T", o.bucket, key, obj)
	}

	return backup, nil
}

func (o *objectStoreBackupAccessor) DeleteBackup() error {
	objects, err := o.objectStore.ListObjects(o.bucket, o.backup+"/")
	if err != nil {
		return err
	}

	var errs []error
	for _, key := range objects {
		o.log.WithFields(logrus.Fields{
			"bucket": o.bucket,
			"key":    key,
		}).Debug("Trying to delete object")
		if err := o.objectStore.DeleteObject(o.bucket, key); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.WithStack(kerrors.NewAggregate(errs))
}

func (o *objectStoreBackupAccessor) SaveRestoreLog(restore string, log io.Reader) error {
	return o.objectStore.PutObject(o.bucket, getRestoreLogKey(o.backup, restore), log)
}

func (o *objectStoreBackupAccessor) SaveRestoreResults(restore string, results io.Reader) error {
	return o.objectStore.PutObject(o.bucket, getRestoreResultsKey(o.backup, restore), results)
}
