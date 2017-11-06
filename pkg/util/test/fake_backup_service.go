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

package test

import (
	"io"

	"github.com/stretchr/testify/mock"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

type FakeBackupService struct {
	mock.Mock
}

func (f *FakeBackupService) GetAllBackups(bucket, path string) ([]*v1.Backup, error) {
	args := f.Called(bucket, path)

	var backups []*v1.Backup

	b := args.Get(0)
	if b != nil {
		backups = b.([]*v1.Backup)
	}

	return backups, args.Error(1)
}

func (f *FakeBackupService) UploadBackup(bucket, path, name string, metadata, backup io.ReadSeeker) error {
	args := f.Called(bucket, path, name, metadata, backup)
	return args.Error(0)
}

func (f *FakeBackupService) DownloadBackup(bucket, path, name string) (io.ReadCloser, error) {
	args := f.Called(bucket, path, name)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (f *FakeBackupService) DeleteBackup(bucket, path, backupName string) error {
	args := f.Called(bucket, path, backupName)
	return args.Error(0)
}

func (f *FakeBackupService) GetBackup(bucket, path, name string) (*v1.Backup, error) {
	var (
		args   = f.Called(bucket, path, name)
		b      = args.Get(0)
		backup *v1.Backup
	)

	if b != nil {
		backup = b.(*v1.Backup)
	}

	return backup, args.Error(1)
}
