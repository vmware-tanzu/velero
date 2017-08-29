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

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/encode"
)

func TestUploadBackup(t *testing.T) {
	tests := []struct {
		name            string
		bucket          string
		bucketExists    bool
		backupName      string
		metadata        io.ReadSeeker
		backup          io.ReadSeeker
		objectStoreErrs map[string]map[string]interface{}
		expectedErr     bool
		expectedRes     map[string][]byte
	}{
		{
			name:         "normal case",
			bucket:       "test-bucket",
			bucketExists: true,
			backupName:   "test-backup",
			metadata:     newStringReadSeeker("foo"),
			backup:       newStringReadSeeker("bar"),
			expectedErr:  false,
			expectedRes: map[string][]byte{
				"test-backup/ark-backup.json":    []byte("foo"),
				"test-backup/test-backup.tar.gz": []byte("bar"),
			},
		},
		{
			name:         "no such bucket causes error",
			bucket:       "test-bucket",
			bucketExists: false,
			backupName:   "test-backup",
			expectedErr:  true,
		},
		{
			name:         "error on metadata upload does not upload data",
			bucket:       "test-bucket",
			bucketExists: true,
			backupName:   "test-backup",
			metadata:     newStringReadSeeker("foo"),
			backup:       newStringReadSeeker("bar"),
			objectStoreErrs: map[string]map[string]interface{}{
				"putobject": map[string]interface{}{
					"test-bucket||test-backup/ark-backup.json": true,
				},
			},
			expectedErr: true,
			expectedRes: make(map[string][]byte),
		},
		{
			name:         "error on data upload deletes metadata",
			bucket:       "test-bucket",
			bucketExists: true,
			backupName:   "test-backup",
			metadata:     newStringReadSeeker("foo"),
			backup:       newStringReadSeeker("bar"),
			objectStoreErrs: map[string]map[string]interface{}{
				"putobject": map[string]interface{}{
					"test-bucket||test-backup/test-backup.tar.gz": true,
				},
			},
			expectedErr: true,
			expectedRes: make(map[string][]byte),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objStore := &fakeObjectStorage{
				returnErrors: test.objectStoreErrs,
				storage:      make(map[string]map[string][]byte),
			}
			if test.bucketExists {
				objStore.storage[test.bucket] = make(map[string][]byte)
			}

			backupService := NewBackupService(objStore)

			err := backupService.UploadBackup(test.bucket, test.backupName, test.metadata, test.backup)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			assert.Equal(t, test.expectedRes, objStore.storage[test.bucket])

		})
	}
}

func TestDownloadBackup(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		backupName  string
		storage     map[string]map[string][]byte
		expectedErr bool
		expectedRes []byte
	}{
		{
			name:       "normal case",
			bucket:     "test-bucket",
			backupName: "test-backup",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{
					"test-backup/test-backup.tar.gz": []byte("foo"),
				},
			},
			expectedErr: false,
			expectedRes: []byte("foo"),
		},
		{
			name:        "no such bucket causes error",
			bucket:      "test-bucket",
			backupName:  "test-backup",
			storage:     map[string]map[string][]byte{},
			expectedErr: true,
		},
		{
			name:       "no such key causes error",
			bucket:     "test-bucket",
			backupName: "test-backup",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{},
			},
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objStore := &fakeObjectStorage{storage: test.storage}
			backupService := NewBackupService(objStore)

			rdr, err := backupService.DownloadBackup(test.bucket, test.backupName)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			if err == nil {
				res, err := ioutil.ReadAll(rdr)
				assert.Nil(t, err)
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}

func TestDeleteBackupFile(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		backupName  string
		storage     map[string]map[string][]byte
		expectedErr bool
		expectedRes map[string][]byte
	}{
		{
			name:       "normal case",
			bucket:     "test-bucket",
			backupName: "bak",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{
					"bak/bak.tar.gz": nil,
				},
			},
			expectedErr: false,
			expectedRes: make(map[string][]byte),
		},
		{
			name:       "failed delete of backup returns error",
			bucket:     "test-bucket",
			backupName: "bak",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{},
			},
			expectedErr: true,
			expectedRes: make(map[string][]byte),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objStore := &fakeObjectStorage{storage: test.storage}
			backupService := NewBackupService(objStore)

			res := backupService.DeleteBackupFile(test.bucket, test.backupName)

			assert.Equal(t, test.expectedErr, res != nil, "got error %v", res)

			assert.Equal(t, test.expectedRes, objStore.storage[test.bucket])
		})
	}
}

func TestDeleteBackupMetadataFile(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		backupName  string
		storage     map[string]map[string][]byte
		expectedErr bool
		expectedRes map[string][]byte
	}{
		{
			name:       "normal case",
			bucket:     "test-bucket",
			backupName: "bak",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{
					"bak/ark-backup.json": nil,
				},
			},
			expectedErr: false,
			expectedRes: make(map[string][]byte),
		},
		{
			name:       "failed delete of file returns error",
			bucket:     "test-bucket",
			backupName: "bak",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{},
			},
			expectedErr: true,
			expectedRes: make(map[string][]byte),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objStore := &fakeObjectStorage{storage: test.storage}
			backupService := NewBackupService(objStore)

			res := backupService.DeleteBackupMetadataFile(test.bucket, test.backupName)

			assert.Equal(t, test.expectedErr, res != nil, "got error %v", res)

			assert.Equal(t, test.expectedRes, objStore.storage[test.bucket])
		})
	}
}

func TestGetAllBackups(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		storage     map[string]map[string][]byte
		expectedRes []*api.Backup
		expectedErr bool
	}{
		{
			name:   "normal case",
			bucket: "test-bucket",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{
					"backup-1/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-1"}}),
					"backup-2/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-2"}}),
				},
			},
			expectedErr: false,
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
			name:   "backup that can't be decoded is ignored",
			bucket: "test-bucket",
			storage: map[string]map[string][]byte{
				"test-bucket": map[string][]byte{
					"backup-1/ark-backup.json": encodeToBytes(&api.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup-1"}}),
					"backup-2/ark-backup.json": []byte("this is not valid backup JSON"),
				},
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
			objStore := &fakeObjectStorage{storage: test.storage}
			backupService := NewBackupService(objStore)

			res, err := backupService.GetAllBackups(test.bucket)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			assert.Equal(t, test.expectedRes, res)
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

type fakeObjectStorage struct {
	storage      map[string]map[string][]byte
	returnErrors map[string]map[string]interface{}
}

func (os *fakeObjectStorage) PutObject(bucket string, key string, body io.ReadSeeker) error {
	if os.returnErrors["putobject"] != nil && os.returnErrors["putobject"][bucket+"||"+key] != nil {
		return errors.New("error")
	}

	if os.storage[bucket] == nil {
		return errors.New("bucket not found")
	}

	data, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	os.storage[bucket][key] = data

	return nil
}

func (os *fakeObjectStorage) GetObject(bucket string, key string) (io.ReadCloser, error) {
	if os.storage == nil {
		return nil, errors.New("storage not initialized")
	}
	if os.storage[bucket] == nil {
		return nil, errors.New("bucket not found")
	}

	if os.storage[bucket][key] == nil {
		return nil, errors.New("key not found")
	}

	return ioutil.NopCloser(bytes.NewReader(os.storage[bucket][key])), nil
}

func (os *fakeObjectStorage) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	if os.storage == nil {
		return nil, errors.New("storage not initialized")
	}
	if os.storage[bucket] == nil {
		return nil, errors.New("bucket not found")
	}

	prefixes := sets.NewString()

	for key := range os.storage[bucket] {
		delimIdx := strings.LastIndex(key, delimiter)

		if delimIdx == -1 {
			prefixes.Insert(key)
		}

		prefixes.Insert(key[0:delimIdx])
	}

	return prefixes.List(), nil
}

func (os *fakeObjectStorage) DeleteObject(bucket string, key string) error {
	if os.storage == nil {
		return errors.New("storage not initialized")
	}
	if os.storage[bucket] == nil {
		return errors.New("bucket not found")
	}

	if _, exists := os.storage[bucket][key]; !exists {
		return errors.New("key not found")
	}

	delete(os.storage[bucket], key)

	return nil
}
