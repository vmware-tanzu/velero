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

func (f *FakeBackupService) GetAllBackups(bucket string) ([]*v1.Backup, error) {
	args := f.Called(bucket)

	var backups []*v1.Backup

	b := args.Get(0)
	if b != nil {
		backups = b.([]*v1.Backup)
	}

	return backups, args.Error(1)
}

func (f *FakeBackupService) UploadBackup(bucket, name string, metadata, backup io.Reader) error {
	args := f.Called(bucket, name, metadata, backup)
	return args.Error(0)
}

func (f *FakeBackupService) DownloadBackup(bucket, name string) (io.ReadCloser, error) {
	args := f.Called(bucket, name)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (f *FakeBackupService) DeleteBackup(bucket, backupName string) error {
	args := f.Called(bucket, backupName)
	return args.Error(0)
}

func (f *FakeBackupService) GetBackup(bucket, name string) (*v1.Backup, error) {
	var (
		args   = f.Called(bucket, name)
		b      = args.Get(0)
		backup *v1.Backup
	)

	if b != nil {
		backup = b.(*v1.Backup)
	}

	return backup, args.Error(1)
}
