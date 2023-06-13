/*
Copyright The Velero Contributors.

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

package kopia

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/content"
	"github.com/kopia/kopia/repo/manifest"
	"github.com/kopia/kopia/repo/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo/mocks"
)

func TestShimRepo(t *testing.T) {
	ctx := context.Background()
	backupRepo := &mocks.BackupRepo{}
	backupRepo.On("Time").Return(time.Time{})
	shim := NewShimRepo(backupRepo)
	// All below calls put together for the implementation are empty or just very simple, and just want to cover testing
	// If wanting to write unit tests for some functions could remove it and with writing new function alone
	shim.VerifyObject(ctx, object.ID{})
	shim.Time()
	shim.ClientOptions()
	shim.Refresh(ctx)
	shim.ContentInfo(ctx, content.ID{})
	shim.PrefetchContents(ctx, []content.ID{}, "hint")
	shim.PrefetchObjects(ctx, []object.ID{}, "hint")
	shim.UpdateDescription("desc")
	shim.NewWriter(ctx, repo.WriteSessionOptions{})
	shim.ReplaceManifests(ctx, map[string]string{}, nil)
	shim.OnSuccessfulFlush(func(ctx context.Context, w repo.RepositoryWriter) error { return nil })

	backupRepo.On("Close", mock.Anything).Return(nil)
	NewShimRepo(backupRepo).Close(ctx)

	var id udmrepo.ID
	backupRepo.On("PutManifest", mock.Anything, mock.Anything).Return(id, nil)
	NewShimRepo(backupRepo).PutManifest(ctx, map[string]string{}, nil)

	var mf manifest.ID
	backupRepo.On("DeleteManifest", mock.Anything, mock.Anything).Return(nil)
	NewShimRepo(backupRepo).DeleteManifest(ctx, mf)

	backupRepo.On("Flush", mock.Anything).Return(nil)
	NewShimRepo(backupRepo).Flush(ctx)

	var objID object.ID
	backupRepo.On("ConcatenateObjects", mock.Anything, mock.Anything).Return(objID)
	NewShimRepo(backupRepo).ConcatenateObjects(ctx, []object.ID{})

	backupRepo.On("NewObjectWriter", mock.Anything, mock.Anything).Return(nil)
	NewShimRepo(backupRepo).NewObjectWriter(ctx, object.WriterOptions{})
}

func TestOpenObject(t *testing.T) {
	tests := []struct {
		name              string
		backupRepo        *mocks.BackupRepo
		isOpenObjectError bool
		isReaderNil       bool
	}{
		{
			name: "Success",
			backupRepo: func() *mocks.BackupRepo {
				backupRepo := &mocks.BackupRepo{}
				backupRepo.On("OpenObject", mock.Anything, mock.Anything).Return(&shimObjectReader{}, nil)
				return backupRepo
			}(),
		},
		{
			name: "Open object error",
			backupRepo: func() *mocks.BackupRepo {
				backupRepo := &mocks.BackupRepo{}
				backupRepo.On("OpenObject", mock.Anything, mock.Anything).Return(&shimObjectReader{}, errors.New("Error open object"))
				return backupRepo
			}(),
			isOpenObjectError: true,
		},
		{
			name: "Get nil reader",
			backupRepo: func() *mocks.BackupRepo {
				backupRepo := &mocks.BackupRepo{}
				backupRepo.On("OpenObject", mock.Anything, mock.Anything).Return(nil, nil)
				return backupRepo
			}(),
			isReaderNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			reader, err := NewShimRepo(tc.backupRepo).OpenObject(ctx, object.ID{})
			if tc.isOpenObjectError {
				assert.Contains(t, err.Error(), "failed to open object")
			} else if tc.isReaderNil {
				assert.Nil(t, reader)
			} else {
				assert.NotNil(t, reader)
				assert.Nil(t, err)
			}
		})
	}
}

func TestFindManifests(t *testing.T) {
	meta := []*udmrepo.ManifestEntryMetadata{}
	tests := []struct {
		name               string
		backupRepo         *mocks.BackupRepo
		isGetManifestError bool
	}{
		{
			name: "Success",
			backupRepo: func() *mocks.BackupRepo {
				backupRepo := &mocks.BackupRepo{}
				backupRepo.On("FindManifests", mock.Anything, mock.Anything).Return(meta, nil)
				return backupRepo
			}(),
		},
		{
			name:               "Failed to find manifest",
			isGetManifestError: true,
			backupRepo: func() *mocks.BackupRepo {
				backupRepo := &mocks.BackupRepo{}
				backupRepo.On("FindManifests", mock.Anything, mock.Anything).Return(meta,
					errors.New("failed to find manifest"))
				return backupRepo
			}(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := NewShimRepo(tc.backupRepo).FindManifests(ctx, map[string]string{})
			if tc.isGetManifestError {
				assert.Contains(t, err.Error(), "failed")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestShimObjReader(t *testing.T) {
	reader := new(shimObjectReader)
	objReader := &mocks.ObjectReader{}
	reader.repoReader = objReader
	// All below calls put together for the implementation are empty or just very simple, and just want to cover testing
	// If wanting to write unit tests for some functions could remove it and with writing new function alone
	objReader.On("Seek", mock.Anything, mock.Anything).Return(int64(0), nil)
	reader.Seek(int64(0), 0)

	objReader.On("Read", mock.Anything).Return(0, nil)
	reader.Read(nil)

	objReader.On("Close").Return(nil)
	reader.Close()

	objReader.On("Length").Return(int64(0))
	reader.Length()
}

func TestShimObjWriter(t *testing.T) {
	writer := new(shimObjectWriter)
	objWriter := &mocks.ObjectWriter{}
	writer.repoWriter = objWriter
	// All below calls put together for the implementation are empty or just very simple, and just want to cover testing
	// If wanting to write unit tests for some functions could remove it and with writing new function alone
	var id udmrepo.ID
	objWriter.On("Checkpoint").Return(id, nil)
	writer.Checkpoint()

	objWriter.On("Result").Return(id, nil)
	writer.Result()

	objWriter.On("Write", mock.Anything).Return(0, nil)
	writer.Write(nil)

	objWriter.On("Close").Return(nil)
	writer.Close()
}
