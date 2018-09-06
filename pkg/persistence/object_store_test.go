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
	"errors"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	cloudprovidermocks "github.com/heptio/ark/pkg/cloudprovider/mocks"
	"github.com/heptio/ark/pkg/util/encode"
	arktest "github.com/heptio/ark/pkg/util/test"
)

type objectBackupStoreTestHarness struct {
	// embedded to reduce verbosity when calling methods
	*objectBackupStore

	objectStore    *cloudprovider.InMemoryObjectStore
	bucket, prefix string
}

func newObjectBackupStoreTestHarness(bucket, prefix string) *objectBackupStoreTestHarness {
	objectStore := cloudprovider.NewInMemoryObjectStore(bucket)

	return &objectBackupStoreTestHarness{
		objectBackupStore: &objectBackupStore{
			objectStore: objectStore,
			bucket:      bucket,
			prefix:      prefix,
			logger:      arktest.NewLogger(),
		},
		objectStore: objectStore,
		bucket:      bucket,
		prefix:      prefix,
	}
}

func TestListBackups(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		storageData cloudprovider.BucketData
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
			name:   "normal case with backup store prefix",
			prefix: "ark-backups/",
			storageData: map[string][]byte{
				"ark-backups/backup-1/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-1"}}),
				"ark-backups/backup-2/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-2"}}),
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("foo", tc.prefix)

			for key, obj := range tc.storageData {
				require.NoError(t, harness.objectStore.PutObject(harness.bucket, key, bytes.NewReader(obj)))
			}

			res, err := harness.ListBackups()

			arktest.AssertErrorMatches(t, tc.expectedErr, err)

			getComparer := func(obj []*api.Backup) func(i, j int) bool {
				return func(i, j int) bool {
					switch strings.Compare(obj[i].Namespace, obj[j].Namespace) {
					case -1:
						return true
					case 1:
						return false
					default:
						// namespaces are the same: compare by name
						return obj[i].Name < obj[j].Name
					}
				}
			}

			sort.Slice(tc.expectedRes, getComparer(tc.expectedRes))
			sort.Slice(res, getComparer(res))

			assert.Equal(t, tc.expectedRes, res)
		})
	}
}

func TestPutBackup(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		metadata     io.Reader
		contents     io.Reader
		log          io.Reader
		expectedErr  string
		expectedKeys []string
	}{
		{
			name:         "normal case",
			metadata:     newStringReadSeeker("metadata"),
			contents:     newStringReadSeeker("contents"),
			log:          newStringReadSeeker("log"),
			expectedErr:  "",
			expectedKeys: []string{"backup-1/ark-backup.json", "backup-1/backup-1.tar.gz", "backup-1/backup-1-logs.gz"},
		},
		{
			name:         "normal case with backup store prefix",
			prefix:       "prefix-1/",
			metadata:     newStringReadSeeker("metadata"),
			contents:     newStringReadSeeker("contents"),
			log:          newStringReadSeeker("log"),
			expectedErr:  "",
			expectedKeys: []string{"prefix-1/backup-1/ark-backup.json", "prefix-1/backup-1/backup-1.tar.gz", "prefix-1/backup-1/backup-1-logs.gz"},
		},
		{
			name:         "error on metadata upload does not upload data",
			metadata:     new(errorReader),
			contents:     newStringReadSeeker("contents"),
			log:          newStringReadSeeker("log"),
			expectedErr:  "error readers return errors",
			expectedKeys: []string{"backup-1/backup-1-logs.gz"},
		},
		{
			name:         "error on data upload deletes metadata",
			metadata:     newStringReadSeeker("metadata"),
			contents:     new(errorReader),
			log:          newStringReadSeeker("log"),
			expectedErr:  "error readers return errors",
			expectedKeys: []string{"backup-1/backup-1-logs.gz"},
		},
		{
			name:         "error on log upload is ok",
			metadata:     newStringReadSeeker("foo"),
			contents:     newStringReadSeeker("bar"),
			log:          new(errorReader),
			expectedErr:  "",
			expectedKeys: []string{"backup-1/ark-backup.json", "backup-1/backup-1.tar.gz"},
		},
		{
			name:         "don't upload data when metadata is nil",
			metadata:     nil,
			contents:     newStringReadSeeker("contents"),
			log:          newStringReadSeeker("log"),
			expectedErr:  "",
			expectedKeys: []string{"backup-1/backup-1-logs.gz"},
		},
	}

	for _, tc := range tests {
		harness := newObjectBackupStoreTestHarness("foo", tc.prefix)

		err := harness.PutBackup("backup-1", tc.metadata, tc.contents, tc.log)

		arktest.AssertErrorMatches(t, tc.expectedErr, err)
		assert.Len(t, harness.objectStore.Data[harness.bucket], len(tc.expectedKeys))
		for _, key := range tc.expectedKeys {
			assert.Contains(t, harness.objectStore.Data[harness.bucket], key)
		}
	}
}

func TestGetBackupContents(t *testing.T) {
	harness := newObjectBackupStoreTestHarness("test-bucket", "")

	harness.objectStore.PutObject(harness.bucket, "test-backup/test-backup.tar.gz", newStringReadSeeker("foo"))

	rc, err := harness.GetBackupContents("test-backup")
	require.NoError(t, err)
	require.NotNil(t, rc)

	data, err := ioutil.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "foo", string(data))
}

func TestDeleteBackup(t *testing.T) {
	tests := []struct {
		name             string
		prefix           string
		listObjectsError error
		deleteErrors     []error
		expectedErr      string
	}{
		{
			name: "normal case",
		},
		{
			name:   "normal case with backup store prefix",
			prefix: "ark-backups/",
		},
		{
			name:         "some delete errors, do as much as we can",
			deleteErrors: []error{errors.New("a"), nil, errors.New("c")},
			expectedErr:  "[a, c]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objectStore := new(cloudprovidermocks.ObjectStore)
			backupStore := &objectBackupStore{
				objectStore: objectStore,
				bucket:      "test-bucket",
				prefix:      test.prefix,
				logger:      arktest.NewLogger(),
			}
			defer objectStore.AssertExpectations(t)

			objects := []string{test.prefix + "bak/ark-backup.json", test.prefix + "bak/bak.tar.gz", test.prefix + "bak/bak.log.gz"}

			objectStore.On("ListObjects", backupStore.bucket, test.prefix+"bak/").Return(objects, test.listObjectsError)
			for i, obj := range objects {
				var err error
				if i < len(test.deleteErrors) {
					err = test.deleteErrors[i]
				}

				objectStore.On("DeleteObject", backupStore.bucket, obj).Return(err)
			}

			err := backupStore.DeleteBackup("bak")

			arktest.AssertErrorMatches(t, test.expectedErr, err)
		})
	}
}

func TestGetDownloadURL(t *testing.T) {
	tests := []struct {
		name        string
		targetKind  api.DownloadTargetKind
		targetName  string
		directory   string
		prefix      string
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
			name:        "backup contents with backup store prefix",
			targetKind:  api.DownloadTargetKindBackupContents,
			targetName:  "my-backup",
			directory:   "my-backup",
			prefix:      "ark-backups/",
			expectedKey: "ark-backups/my-backup/my-backup.tar.gz",
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
			harness := newObjectBackupStoreTestHarness("test-bucket", test.prefix)

			require.NoError(t, harness.objectStore.PutObject("test-bucket", test.expectedKey, newStringReadSeeker("foo")))

			url, err := harness.GetDownloadURL(test.directory, api.DownloadTarget{Kind: test.targetKind, Name: test.targetName})
			require.NoError(t, err)
			assert.Equal(t, "a-url", url)
		})
	}
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

type errorReader struct{}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, errors.New("error readers return errors")
}
