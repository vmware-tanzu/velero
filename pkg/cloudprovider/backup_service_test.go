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
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	testutil "github.com/heptio/ark/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/encode"
)

func TestUploadBackup(t *testing.T) {
	tests := []struct {
		name                 string
		metadata             io.ReadSeeker
		metadataError        error
		expectMetadataDelete bool
		backup               io.ReadSeeker
		backupError          error
		log                  io.ReadSeeker
		logError             error
		expectedErr          string
	}{
		{
			name:     "normal case",
			metadata: newStringReadSeeker("foo"),
			backup:   newStringReadSeeker("bar"),
			log:      newStringReadSeeker("baz"),
		},
		{
			name:          "error on metadata upload does not upload data or log",
			metadata:      newStringReadSeeker("foo"),
			metadataError: errors.New("md"),
			expectedErr:   "md",
		},
		{
			name:                 "error on data upload deletes metadata",
			metadata:             newStringReadSeeker("foo"),
			backup:               newStringReadSeeker("bar"),
			backupError:          errors.New("backup"),
			expectMetadataDelete: true,
			expectedErr:          "backup",
		},
		{
			name:     "error on log upload is ok",
			metadata: newStringReadSeeker("foo"),
			backup:   newStringReadSeeker("bar"),
			log:      newStringReadSeeker("baz"),
			logError: errors.New("log"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objStore := &testutil.ObjectStorageAdapter{}

			bucket := "test-bucket"
			backupName := "test-backup"

			if test.metadata != nil {
				objStore.On("PutObject", bucket, backupName+"/ark-backup.json", test.metadata).Return(test.metadataError)
			}
			if test.backup != nil {
				objStore.On("PutObject", bucket, backupName+"/"+backupName+".tar.gz", test.backup).Return(test.backupError)
			}
			if test.log != nil {
				objStore.On("PutObject", bucket, backupName+"/"+backupName+".log.gz", test.log).Return(test.logError)
			}
			if test.expectMetadataDelete {
				objStore.On("DeleteObject", bucket, backupName+"/ark-backup.json").Return(nil)
			}

			backupService := NewBackupService(objStore)

			err := backupService.UploadBackup(bucket, backupName, test.metadata, test.backup, test.log)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			objStore.AssertExpectations(t)
		})
	}
}

func TestDownloadBackup(t *testing.T) {
	o := &testutil.ObjectStorageAdapter{}
	bucket := "b"
	backup := "bak"
	o.On("GetObject", bucket, backup+"/"+backup+".tar.gz").Return(ioutil.NopCloser(strings.NewReader("foo")), nil)
	s := NewBackupService(o)
	rc, err := s.DownloadBackup(bucket, backup)
	require.NoError(t, err)
	require.NotNil(t, rc)
	data, err := ioutil.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "foo", string(data))
	o.AssertExpectations(t)
}

func TestDeleteBackup(t *testing.T) {
	tests := []struct {
		name             string
		listObjectsError error
		deleteErrors     []error
		expectedErr      string
	}{
		{
			name: "normal case",
		},
		{
			name:         "some delete errors, do as much as we can",
			deleteErrors: []error{errors.New("a"), nil, errors.New("c")},
			expectedErr:  "[a, c]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bucket := "bucket"
			backup := "bak"
			objects := []string{"bak/ark-backup.json", "bak/bak.tar.gz", "bak/bak.log.gz"}

			objStore := &testutil.ObjectStorageAdapter{}
			objStore.On("ListObjects", bucket, backup+"/").Return(objects, test.listObjectsError)
			for i, o := range objects {
				var err error
				if i < len(test.deleteErrors) {
					err = test.deleteErrors[i]
				}

				objStore.On("DeleteObject", bucket, o).Return(err)
			}

			backupService := NewBackupService(objStore)

			err := backupService.DeleteBackupDir(bucket, backup)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			objStore.AssertExpectations(t)
		})
	}
}

func TestGetAllBackups(t *testing.T) {
	tests := []struct {
		name        string
		storageData map[string][]byte
		expectedRes []*api.Backup
		expectedErr string
	}{
		{
			name: "normal case",
			storageData: map[string][]byte{
				"backup-1/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-1"}}),
				"backup-2/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-2"}}),
			},
			expectedRes: []*api.Backup{
				&api.Backup{
					TypeMeta:   metav1.TypeMeta{Kind: "Backup", APIVersion: "ark.heptio.com/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "backup-1"},
				},
				&api.Backup{
					TypeMeta:   metav1.TypeMeta{Kind: "Backup", APIVersion: "ark.heptio.com/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "backup-2"},
				},
			},
		},
		{
			name: "backup that can't be decoded is ignored",
			storageData: map[string][]byte{
				"backup-1/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-1"}}),
				"backup-2/ark-backup.json": []byte("this is not valid backup JSON"),
			},
			expectedRes: []*api.Backup{
				&api.Backup{
					TypeMeta:   metav1.TypeMeta{Kind: "Backup", APIVersion: "ark.heptio.com/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "backup-1"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bucket := "bucket"

			objStore := &testutil.ObjectStorageAdapter{}
			objStore.On("ListCommonPrefixes", bucket, "/").Return([]string{"backup-1", "backup-2"}, nil)
			objStore.On("GetObject", bucket, "backup-1/ark-backup.json").Return(ioutil.NopCloser(bytes.NewReader(test.storageData["backup-1/ark-backup.json"])), nil)
			objStore.On("GetObject", bucket, "backup-2/ark-backup.json").Return(ioutil.NopCloser(bytes.NewReader(test.storageData["backup-2/ark-backup.json"])), nil)

			backupService := NewBackupService(objStore)

			res, err := backupService.GetAllBackups(bucket)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedRes, res)

			objStore.AssertExpectations(t)
		})
	}
}

func jsonMarshal(obj interface{}) []byte {
	res, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return res
}

func encodeToBytes(obj runtime.Object) []byte {
	res, err := encode.Encode(obj, "json")
	if err != nil {
		panic(err)
	}
	return res
}

type stringReadSeeker struct {
	*strings.Reader
}

func newStringReadSeeker(s string) *stringReadSeeker {
	return &stringReadSeeker{
		Reader: strings.NewReader(s),
	}
}

func (srs *stringReadSeeker) Seek(offset int64, whence int) (int64, error) {
	panic("not implemented")
}
