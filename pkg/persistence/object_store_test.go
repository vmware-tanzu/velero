/*
Copyright 2017 the Velero contributors.

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
	"compress/gzip"
	"encoding/json"
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

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	providermocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type objectBackupStoreTestHarness struct {
	// embedded to reduce verbosity when calling methods
	*objectBackupStore

	objectStore    *inMemoryObjectStore
	bucket, prefix string
}

func newObjectBackupStoreTestHarness(bucket, prefix string) *objectBackupStoreTestHarness {
	objectStore := newInMemoryObjectStore(bucket)

	return &objectBackupStoreTestHarness{
		objectBackupStore: &objectBackupStore{
			objectStore: objectStore,
			bucket:      bucket,
			layout:      NewObjectStoreLayout(prefix),
			logger:      velerotest.NewLogger(),
		},
		objectStore: objectStore,
		bucket:      bucket,
		prefix:      prefix,
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		storageData BucketData
		expectErr   bool
	}{
		{
			name:      "empty backup store with no prefix is valid",
			expectErr: false,
		},
		{
			name:      "empty backup store with a prefix is valid",
			prefix:    "bar",
			expectErr: false,
		},
		{
			name: "backup store with no prefix and only unsupported directories is invalid",
			storageData: map[string][]byte{
				"backup-1/velero-backup.json": {},
				"backup-2/velero-backup.json": {},
			},
			expectErr: true,
		},
		{
			name:   "backup store with a prefix and only unsupported directories is invalid",
			prefix: "backups",
			storageData: map[string][]byte{
				"backups/backup-1/velero-backup.json": {},
				"backups/backup-2/velero-backup.json": {},
			},
			expectErr: true,
		},
		{
			name: "backup store with no prefix and both supported and unsupported directories is invalid",
			storageData: map[string][]byte{
				"backups/backup-1/velero-backup.json": {},
				"backups/backup-2/velero-backup.json": {},
				"restores/restore-1/foo":              {},
				"unsupported-dir/foo":                 {},
			},
			expectErr: true,
		},
		{
			name:   "backup store with a prefix and both supported and unsupported directories is invalid",
			prefix: "cluster-1",
			storageData: map[string][]byte{
				"cluster-1/backups/backup-1/velero-backup.json": {},
				"cluster-1/backups/backup-2/velero-backup.json": {},
				"cluster-1/restores/restore-1/foo":              {},
				"cluster-1/unsupported-dir/foo":                 {},
			},
			expectErr: true,
		},
		{
			name: "backup store with no prefix and only supported directories is valid",
			storageData: map[string][]byte{
				"backups/backup-1/velero-backup.json": {},
				"backups/backup-2/velero-backup.json": {},
				"restores/restore-1/foo":              {},
			},
			expectErr: false,
		},
		{
			name:   "backup store with a prefix and only supported directories is valid",
			prefix: "cluster-1",
			storageData: map[string][]byte{
				"cluster-1/backups/backup-1/velero-backup.json": {},
				"cluster-1/backups/backup-2/velero-backup.json": {},
				"cluster-1/restores/restore-1/foo":              {},
			},
			expectErr: false,
		},
		{
			name: "backup store with plugins directory is valid",
			storageData: map[string][]byte{
				"plugins/vsphere/foo": {},
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("foo", tc.prefix)

			for key, obj := range tc.storageData {
				require.NoError(t, harness.objectStore.PutObject(harness.bucket, key, bytes.NewReader(obj)))
			}

			err := harness.IsValid()
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListBackups(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		storageData BucketData
		expectedRes []string
		expectedErr string
	}{
		{
			name: "normal case",
			storageData: map[string][]byte{
				"backups/backup-1/velero-backup.json": encodeToBytes(builder.ForBackup("", "backup-1").Result()),
				"backups/backup-2/velero-backup.json": encodeToBytes(builder.ForBackup("", "backup-2").Result()),
			},
			expectedRes: []string{"backup-1", "backup-2"},
		},
		{
			name:   "normal case with backup store prefix",
			prefix: "velero-backups/",
			storageData: map[string][]byte{
				"velero-backups/backups/backup-1/velero-backup.json": encodeToBytes(builder.ForBackup("", "backup-1").Result()),
				"velero-backups/backups/backup-2/velero-backup.json": encodeToBytes(builder.ForBackup("", "backup-2").Result()),
			},
			expectedRes: []string{"backup-1", "backup-2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("foo", tc.prefix)

			for key, obj := range tc.storageData {
				require.NoError(t, harness.objectStore.PutObject(harness.bucket, key, bytes.NewReader(obj)))
			}

			res, err := harness.ListBackups()

			velerotest.AssertErrorMatches(t, tc.expectedErr, err)

			sort.Strings(tc.expectedRes)
			sort.Strings(res)

			assert.Equal(t, tc.expectedRes, res)
		})
	}
}

func TestPutBackup(t *testing.T) {
	tests := []struct {
		name            string
		prefix          string
		metadata        io.Reader
		contents        io.Reader
		log             io.Reader
		podVolumeBackup io.Reader
		snapshots       io.Reader
		resourceList    io.Reader
		expectedErr     string
		expectedKeys    []string
	}{
		{
			name:            "normal case",
			metadata:        newStringReadSeeker("metadata"),
			contents:        newStringReadSeeker("contents"),
			log:             newStringReadSeeker("log"),
			podVolumeBackup: newStringReadSeeker("podVolumeBackup"),
			snapshots:       newStringReadSeeker("snapshots"),
			resourceList:    newStringReadSeeker("resourceList"),
			expectedErr:     "",
			expectedKeys: []string{
				"backups/backup-1/velero-backup.json",
				"backups/backup-1/backup-1.tar.gz",
				"backups/backup-1/backup-1-logs.gz",
				"backups/backup-1/backup-1-podvolumebackups.json.gz",
				"backups/backup-1/backup-1-volumesnapshots.json.gz",
				"backups/backup-1/backup-1-resource-list.json.gz",
			},
		},
		{
			name:            "normal case with backup store prefix",
			prefix:          "prefix-1/",
			metadata:        newStringReadSeeker("metadata"),
			contents:        newStringReadSeeker("contents"),
			log:             newStringReadSeeker("log"),
			podVolumeBackup: newStringReadSeeker("podVolumeBackup"),
			snapshots:       newStringReadSeeker("snapshots"),
			resourceList:    newStringReadSeeker("resourceList"),
			expectedErr:     "",
			expectedKeys: []string{
				"prefix-1/backups/backup-1/velero-backup.json",
				"prefix-1/backups/backup-1/backup-1.tar.gz",
				"prefix-1/backups/backup-1/backup-1-logs.gz",
				"prefix-1/backups/backup-1/backup-1-podvolumebackups.json.gz",
				"prefix-1/backups/backup-1/backup-1-volumesnapshots.json.gz",
				"prefix-1/backups/backup-1/backup-1-resource-list.json.gz",
			},
		},
		{
			name:            "error on metadata upload does not upload data",
			metadata:        new(errorReader),
			contents:        newStringReadSeeker("contents"),
			log:             newStringReadSeeker("log"),
			podVolumeBackup: newStringReadSeeker("podVolumeBackup"),
			snapshots:       newStringReadSeeker("snapshots"),
			resourceList:    newStringReadSeeker("resourceList"),
			expectedErr:     "error readers return errors",
			expectedKeys:    []string{"backups/backup-1/backup-1-logs.gz"},
		},
		{
			name:         "error on data upload deletes metadata",
			metadata:     newStringReadSeeker("metadata"),
			contents:     new(errorReader),
			log:          newStringReadSeeker("log"),
			snapshots:    newStringReadSeeker("snapshots"),
			resourceList: newStringReadSeeker("resourceList"),
			expectedErr:  "error readers return errors",
			expectedKeys: []string{"backups/backup-1/backup-1-logs.gz"},
		},
		{
			name:            "error on log upload is ok",
			metadata:        newStringReadSeeker("foo"),
			contents:        newStringReadSeeker("bar"),
			log:             new(errorReader),
			podVolumeBackup: newStringReadSeeker("podVolumeBackup"),
			snapshots:       newStringReadSeeker("snapshots"),
			resourceList:    newStringReadSeeker("resourceList"),
			expectedErr:     "",
			expectedKeys: []string{
				"backups/backup-1/velero-backup.json",
				"backups/backup-1/backup-1.tar.gz",
				"backups/backup-1/backup-1-podvolumebackups.json.gz",
				"backups/backup-1/backup-1-volumesnapshots.json.gz",
				"backups/backup-1/backup-1-resource-list.json.gz",
			},
		},
		{
			name:            "don't upload data when metadata is nil",
			metadata:        nil,
			contents:        newStringReadSeeker("contents"),
			log:             newStringReadSeeker("log"),
			podVolumeBackup: newStringReadSeeker("podVolumeBackup"),
			snapshots:       newStringReadSeeker("snapshots"),
			resourceList:    newStringReadSeeker("resourceList"),
			expectedErr:     "",
			expectedKeys:    []string{"backups/backup-1/backup-1-logs.gz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("foo", tc.prefix)

			backupInfo := BackupInfo{
				Name:               "backup-1",
				Metadata:           tc.metadata,
				Contents:           tc.contents,
				Log:                tc.log,
				PodVolumeBackups:   tc.podVolumeBackup,
				VolumeSnapshots:    tc.snapshots,
				BackupResourceList: tc.resourceList,
			}
			err := harness.PutBackup(backupInfo)

			velerotest.AssertErrorMatches(t, tc.expectedErr, err)
			assert.Len(t, harness.objectStore.Data[harness.bucket], len(tc.expectedKeys))
			for _, key := range tc.expectedKeys {
				assert.Contains(t, harness.objectStore.Data[harness.bucket], key)
			}
		})
	}
}

func TestGetBackupMetadata(t *testing.T) {
	tests := []struct {
		name       string
		backupName string
		key        string
		obj        metav1.Object
		wantErr    error
	}{
		{
			name:       "metadata file returns correctly",
			backupName: "foo",
			key:        "backups/foo/velero-backup.json",
			obj:        builder.ForBackup(velerov1api.DefaultNamespace, "foo").Result(),
		},
		{
			name:       "no metadata file returns an error",
			backupName: "foo",
			wantErr:    errors.New("key not found"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("test-bucket", "")

			if tc.obj != nil {
				jsonBytes, err := json.Marshal(tc.obj)
				require.NoError(t, err)

				require.NoError(t, harness.objectStore.PutObject(harness.bucket, tc.key, bytes.NewReader(jsonBytes)))
			}

			res, err := harness.GetBackupMetadata(tc.backupName)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)

				assert.Equal(t, tc.obj.GetNamespace(), res.Namespace)
				assert.Equal(t, tc.obj.GetName(), res.Name)
			}
		})
	}
}

func TestGetBackupVolumeSnapshots(t *testing.T) {
	harness := newObjectBackupStoreTestHarness("test-bucket", "")

	// volumesnapshots file not found should not error
	harness.objectStore.PutObject(harness.bucket, "backups/test-backup/velero-backup.json", newStringReadSeeker("foo"))
	res, err := harness.GetBackupVolumeSnapshots("test-backup")
	assert.NoError(t, err)
	assert.Nil(t, res)

	// volumesnapshots file containing invalid data should error
	harness.objectStore.PutObject(harness.bucket, "backups/test-backup/test-backup-volumesnapshots.json.gz", newStringReadSeeker("foo"))
	res, err = harness.GetBackupVolumeSnapshots("test-backup")
	assert.NotNil(t, err)

	// volumesnapshots file containing gzipped json data should return correctly
	snapshots := []*volume.Snapshot{
		{
			Spec: volume.SnapshotSpec{
				BackupName:           "test-backup",
				PersistentVolumeName: "pv-1",
			},
		},
		{
			Spec: volume.SnapshotSpec{
				BackupName:           "test-backup",
				PersistentVolumeName: "pv-2",
			},
		},
	}

	obj := new(bytes.Buffer)
	gzw := gzip.NewWriter(obj)

	require.NoError(t, json.NewEncoder(gzw).Encode(snapshots))
	require.NoError(t, gzw.Close())
	require.NoError(t, harness.objectStore.PutObject(harness.bucket, "backups/test-backup/test-backup-volumesnapshots.json.gz", obj))

	res, err = harness.GetBackupVolumeSnapshots("test-backup")
	assert.NoError(t, err)
	assert.EqualValues(t, snapshots, res)
}

func TestGetBackupContents(t *testing.T) {
	harness := newObjectBackupStoreTestHarness("test-bucket", "")

	harness.objectStore.PutObject(harness.bucket, "backups/test-backup/test-backup.tar.gz", newStringReadSeeker("foo"))

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
			prefix: "velero-backups/",
		},
		{
			name:         "some delete errors, do as much as we can",
			deleteErrors: []error{errors.New("a"), nil, errors.New("c")},
			expectedErr:  "[a, c]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objectStore := new(providermocks.ObjectStore)
			backupStore := &objectBackupStore{
				objectStore: objectStore,
				bucket:      "test-bucket",
				layout:      NewObjectStoreLayout(test.prefix),
				logger:      velerotest.NewLogger(),
			}
			defer objectStore.AssertExpectations(t)

			objects := []string{test.prefix + "backups/bak/velero-backup.json", test.prefix + "backups/bak/bak.tar.gz", test.prefix + "backups/bak/bak.log.gz"}

			objectStore.On("ListObjects", backupStore.bucket, test.prefix+"backups/bak/").Return(objects, test.listObjectsError)
			for i, obj := range objects {
				var err error
				if i < len(test.deleteErrors) {
					err = test.deleteErrors[i]
				}

				objectStore.On("DeleteObject", backupStore.bucket, obj).Return(err)
			}

			err := backupStore.DeleteBackup("bak")

			velerotest.AssertErrorMatches(t, test.expectedErr, err)
		})
	}
}

func TestGetDownloadURL(t *testing.T) {
	tests := []struct {
		name              string
		targetName        string
		expectedKeyByKind map[velerov1api.DownloadTargetKind]string
		prefix            string
	}{
		{
			name:       "backup",
			targetName: "my-backup",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindBackupContents:        "backups/my-backup/my-backup.tar.gz",
				velerov1api.DownloadTargetKindBackupLog:             "backups/my-backup/my-backup-logs.gz",
				velerov1api.DownloadTargetKindBackupVolumeSnapshots: "backups/my-backup/my-backup-volumesnapshots.json.gz",
				velerov1api.DownloadTargetKindBackupResourceList:    "backups/my-backup/my-backup-resource-list.json.gz",
			},
		},
		{
			name:       "backup with prefix",
			targetName: "my-backup",
			prefix:     "velero-backups/",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindBackupContents:        "velero-backups/backups/my-backup/my-backup.tar.gz",
				velerov1api.DownloadTargetKindBackupLog:             "velero-backups/backups/my-backup/my-backup-logs.gz",
				velerov1api.DownloadTargetKindBackupVolumeSnapshots: "velero-backups/backups/my-backup/my-backup-volumesnapshots.json.gz",
				velerov1api.DownloadTargetKindBackupResourceList:    "velero-backups/backups/my-backup/my-backup-resource-list.json.gz",
			},
		},
		{
			name:       "backup with multiple dashes",
			targetName: "b-cool-20170913154901-20170913154902",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindBackupContents:        "backups/b-cool-20170913154901-20170913154902/b-cool-20170913154901-20170913154902.tar.gz",
				velerov1api.DownloadTargetKindBackupLog:             "backups/b-cool-20170913154901-20170913154902/b-cool-20170913154901-20170913154902-logs.gz",
				velerov1api.DownloadTargetKindBackupVolumeSnapshots: "backups/b-cool-20170913154901-20170913154902/b-cool-20170913154901-20170913154902-volumesnapshots.json.gz",
				velerov1api.DownloadTargetKindBackupResourceList:    "backups/b-cool-20170913154901-20170913154902/b-cool-20170913154901-20170913154902-resource-list.json.gz",
			},
		},
		{
			name:       "scheduled backup",
			targetName: "my-backup-20170913154901",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindBackupContents:        "backups/my-backup-20170913154901/my-backup-20170913154901.tar.gz",
				velerov1api.DownloadTargetKindBackupLog:             "backups/my-backup-20170913154901/my-backup-20170913154901-logs.gz",
				velerov1api.DownloadTargetKindBackupVolumeSnapshots: "backups/my-backup-20170913154901/my-backup-20170913154901-volumesnapshots.json.gz",
				velerov1api.DownloadTargetKindBackupResourceList:    "backups/my-backup-20170913154901/my-backup-20170913154901-resource-list.json.gz",
			},
		},
		{
			name:       "scheduled backup with prefix",
			targetName: "my-backup-20170913154901",
			prefix:     "velero-backups/",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindBackupContents:        "velero-backups/backups/my-backup-20170913154901/my-backup-20170913154901.tar.gz",
				velerov1api.DownloadTargetKindBackupLog:             "velero-backups/backups/my-backup-20170913154901/my-backup-20170913154901-logs.gz",
				velerov1api.DownloadTargetKindBackupVolumeSnapshots: "velero-backups/backups/my-backup-20170913154901/my-backup-20170913154901-volumesnapshots.json.gz",
				velerov1api.DownloadTargetKindBackupResourceList:    "velero-backups/backups/my-backup-20170913154901/my-backup-20170913154901-resource-list.json.gz",
			},
		},
		{
			name:       "restore",
			targetName: "my-backup",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindRestoreLog:     "restores/my-backup/restore-my-backup-logs.gz",
				velerov1api.DownloadTargetKindRestoreResults: "restores/my-backup/restore-my-backup-results.gz",
			},
		},
		{
			name:       "restore with prefix",
			targetName: "my-backup",
			prefix:     "velero-backups/",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindRestoreLog:     "velero-backups/restores/my-backup/restore-my-backup-logs.gz",
				velerov1api.DownloadTargetKindRestoreResults: "velero-backups/restores/my-backup/restore-my-backup-results.gz",
			},
		},
		{
			name:       "restore with multiple dashes",
			targetName: "b-cool-20170913154901-20170913154902",
			expectedKeyByKind: map[velerov1api.DownloadTargetKind]string{
				velerov1api.DownloadTargetKindRestoreLog:     "restores/b-cool-20170913154901-20170913154902/restore-b-cool-20170913154901-20170913154902-logs.gz",
				velerov1api.DownloadTargetKindRestoreResults: "restores/b-cool-20170913154901-20170913154902/restore-b-cool-20170913154901-20170913154902-results.gz",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newObjectBackupStoreTestHarness("test-bucket", test.prefix)

			for kind, expectedKey := range test.expectedKeyByKind {
				t.Run(string(kind), func(t *testing.T) {
					require.NoError(t, harness.objectStore.PutObject("test-bucket", expectedKey, newStringReadSeeker("foo")))

					url, err := harness.GetDownloadURL(velerov1api.DownloadTarget{Kind: kind, Name: test.targetName})
					require.NoError(t, err)
					assert.Equal(t, "a-url", url)
				})
			}
		})
	}
}

type objectStoreGetter map[string]velero.ObjectStore

func (osg objectStoreGetter) GetObjectStore(provider string) (velero.ObjectStore, error) {
	res, ok := osg[provider]
	if !ok {
		return nil, errors.New("object store not found")
	}

	return res, nil
}

// TestNewObjectBackupStore runs the NewObjectBackupStore constructor and ensures
// that an ObjectBackupStore is constructed correctly or an appropriate error is
// returned.
func TestNewObjectBackupStore(t *testing.T) {
	tests := []struct {
		name              string
		location          *velerov1api.BackupStorageLocation
		objectStoreGetter objectStoreGetter
		wantBucket        string
		wantPrefix        string
		wantErr           string
	}{
		{
			name:     "location with no ObjectStorage field results in an error",
			location: new(velerov1api.BackupStorageLocation),
			wantErr:  "backup storage location does not use object storage",
		},
		{
			name:     "location with no Provider field results in an error",
			location: builder.ForBackupStorageLocation("", "").Bucket("").Result(),
			wantErr:  "object storage provider name must not be empty",
		},
		{
			name:     "location with a Bucket field with a '/' in the middle results in an error",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("invalid/bucket").Result(),
			wantErr:  "backup storage location's bucket name \"invalid/bucket\" must not contain a '/' (if using a prefix, put it in the 'Prefix' field instead)",
		},
		{
			name:     "when Bucket has a leading and trailing slash, they are both stripped",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("/bucket/").Result(),
			objectStoreGetter: objectStoreGetter{
				"provider-1": newInMemoryObjectStore("bucket"),
			},
			wantBucket: "bucket",
		},
		{
			name:     "when Prefix has a leading and trailing slash, the leading slash is stripped and the trailing slash is left",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("bucket").Prefix("/prefix/").Result(),
			objectStoreGetter: objectStoreGetter{
				"provider-1": newInMemoryObjectStore("bucket"),
			},
			wantBucket: "bucket",
			wantPrefix: "prefix/",
		},
		{
			name:     "when Prefix has no leading or trailing slash, a trailing slash is added",
			location: builder.ForBackupStorageLocation("", "").Provider("provider-1").Bucket("bucket").Prefix("prefix").Result(),
			objectStoreGetter: objectStoreGetter{
				"provider-1": newInMemoryObjectStore("bucket"),
			},
			wantBucket: "bucket",
			wantPrefix: "prefix/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := NewObjectBackupStore(tc.location, tc.objectStoreGetter, velerotest.NewLogger())
			if tc.wantErr != "" {
				require.Equal(t, tc.wantErr, err.Error())
			} else {
				require.Nil(t, err)

				store, ok := res.(*objectBackupStore)
				require.True(t, ok)

				assert.Equal(t, tc.wantBucket, store.bucket)
				assert.Equal(t, tc.wantPrefix, store.layout.rootPrefix)
			}
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
