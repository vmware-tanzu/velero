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

package persistence

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	testutil "github.com/heptio/ark/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/encode"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestUploadBackup(t *testing.T) {
	tests := []struct {
		name                 string
		metadata             io.ReadSeeker
		metadataError        error
		expectMetadataDelete bool
		backup               io.ReadSeeker
		backupError          error
		expectBackupUpload   bool
		log                  io.ReadSeeker
		logError             error
		expectedErr          string
	}{
		{
			name:               "normal case",
			metadata:           newStringReadSeeker("foo"),
			backup:             newStringReadSeeker("bar"),
			expectBackupUpload: true,
			log:                newStringReadSeeker("baz"),
		},
		{
			name:          "error on metadata upload does not upload data",
			metadata:      newStringReadSeeker("foo"),
			metadataError: errors.New("md"),
			log:           newStringReadSeeker("baz"),
			expectedErr:   "md",
		},
		{
			name:                 "error on data upload deletes metadata",
			metadata:             newStringReadSeeker("foo"),
			backup:               newStringReadSeeker("bar"),
			expectBackupUpload:   true,
			backupError:          errors.New("backup"),
			expectMetadataDelete: true,
			expectedErr:          "backup",
		},
		{
			name:               "error on log upload is ok",
			metadata:           newStringReadSeeker("foo"),
			backup:             newStringReadSeeker("bar"),
			expectBackupUpload: true,
			log:                newStringReadSeeker("baz"),
			logError:           errors.New("log"),
		},
		{
			name:   "don't upload data when metadata is nil",
			backup: newStringReadSeeker("bar"),
			log:    newStringReadSeeker("baz"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				objectStore = &testutil.ObjectStore{}
				bucket      = "test-bucket"
				backupName  = "test-backup"
				logger      = arktest.NewLogger()
			)
			defer objectStore.AssertExpectations(t)

			if test.metadata != nil {
				objectStore.On("PutObject", bucket, backupName+"/ark-backup.json", test.metadata).Return(test.metadataError)
			}
			if test.backup != nil && test.expectBackupUpload {
				objectStore.On("PutObject", bucket, backupName+"/"+backupName+".tar.gz", test.backup).Return(test.backupError)
			}
			if test.log != nil {
				objectStore.On("PutObject", bucket, backupName+"/"+backupName+"-logs.gz", test.log).Return(test.logError)
			}
			if test.expectMetadataDelete {
				objectStore.On("DeleteObject", bucket, backupName+"/ark-backup.json").Return(nil)
			}

			err := UploadBackup(logger, objectStore, bucket, backupName, test.metadata, test.backup, test.log)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestDownloadBackup(t *testing.T) {
	var (
		objectStore = &testutil.ObjectStore{}
		bucket      = "b"
		backup      = "bak"
	)
	objectStore.On("GetObject", bucket, backup+"/"+backup+".tar.gz").Return(ioutil.NopCloser(strings.NewReader("foo")), nil)

	rc, err := DownloadBackup(objectStore, bucket, backup)
	require.NoError(t, err)
	require.NotNil(t, rc)
	data, err := ioutil.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "foo", string(data))
	objectStore.AssertExpectations(t)
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
			var (
				bucket      = "bucket"
				backup      = "bak"
				objects     = []string{"bak/ark-backup.json", "bak/bak.tar.gz", "bak/bak.log.gz"}
				objectStore = &testutil.ObjectStore{}
				logger      = arktest.NewLogger()
			)

			objectStore.On("ListObjects", bucket, backup+"/").Return(objects, test.listObjectsError)
			for i, obj := range objects {
				var err error
				if i < len(test.deleteErrors) {
					err = test.deleteErrors[i]
				}

				objectStore.On("DeleteObject", bucket, obj).Return(err)
			}

			err := DeleteBackupDir(logger, objectStore, bucket, backup)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			objectStore.AssertExpectations(t)
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
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Backup", APIVersion: "ark.heptio.com/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "backup-1"},
				},
				{
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
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Backup", APIVersion: "ark.heptio.com/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "backup-1"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				bucket      = "bucket"
				objectStore = &testutil.ObjectStore{}
				logger      = arktest.NewLogger()
			)

			objectStore.On("ListCommonPrefixes", bucket, "/").Return([]string{"backup-1", "backup-2"}, nil)
			objectStore.On("GetObject", bucket, "backup-1/ark-backup.json").Return(ioutil.NopCloser(bytes.NewReader(test.storageData["backup-1/ark-backup.json"])), nil)
			objectStore.On("GetObject", bucket, "backup-2/ark-backup.json").Return(ioutil.NopCloser(bytes.NewReader(test.storageData["backup-2/ark-backup.json"])), nil)

			res, err := ListBackups(logger, objectStore, bucket)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedRes, res)

			objectStore.AssertExpectations(t)
		})
	}
}

func TestCreateSignedURL(t *testing.T) {
	tests := []struct {
		name        string
		targetKind  api.DownloadTargetKind
		targetName  string
		directory   string
		expectedKey string
	}{
		{
			name:        "backup contents",
			targetKind:  api.DownloadTargetKindBackupContents,
			targetName:  "my-backup",
			directory:   "my-backup",
			expectedKey: "my-backup/my-backup.tar.gz",
		},
		{
			name:        "backup log",
			targetKind:  api.DownloadTargetKindBackupLog,
			targetName:  "my-backup",
			directory:   "my-backup",
			expectedKey: "my-backup/my-backup-logs.gz",
		},
		{
			name:        "scheduled backup contents",
			targetKind:  api.DownloadTargetKindBackupContents,
			targetName:  "my-backup-20170913154901",
			directory:   "my-backup-20170913154901",
			expectedKey: "my-backup-20170913154901/my-backup-20170913154901.tar.gz",
		},
		{
			name:        "scheduled backup log",
			targetKind:  api.DownloadTargetKindBackupLog,
			targetName:  "my-backup-20170913154901",
			directory:   "my-backup-20170913154901",
			expectedKey: "my-backup-20170913154901/my-backup-20170913154901-logs.gz",
		},
		{
			name:        "restore log",
			targetKind:  api.DownloadTargetKindRestoreLog,
			targetName:  "b-20170913154901",
			directory:   "b",
			expectedKey: "b/restore-b-20170913154901-logs.gz",
		},
		{
			name:        "restore results",
			targetKind:  api.DownloadTargetKindRestoreResults,
			targetName:  "b-20170913154901",
			directory:   "b",
			expectedKey: "b/restore-b-20170913154901-results.gz",
		},
		{
			name:        "restore results - backup has multiple dashes (e.g. restore of scheduled backup)",
			targetKind:  api.DownloadTargetKindRestoreResults,
			targetName:  "b-cool-20170913154901-20170913154902",
			directory:   "b-cool-20170913154901",
			expectedKey: "b-cool-20170913154901/restore-b-cool-20170913154901-20170913154902-results.gz",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				objectStore = &testutil.ObjectStore{}
			)
			defer objectStore.AssertExpectations(t)

			target := api.DownloadTarget{
				Kind: test.targetKind,
				Name: test.targetName,
			}
			objectStore.On("CreateSignedURL", "bucket", test.expectedKey, time.Duration(0)).Return("url", nil)
			url, err := CreateSignedURL(objectStore, target, "bucket", test.directory, 0)
			require.NoError(t, err)
			assert.Equal(t, "url", url)
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
	return 0, nil
}
