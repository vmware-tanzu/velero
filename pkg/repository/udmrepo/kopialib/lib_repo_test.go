/*
Copyright the Velero contributors.

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

package kopialib

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kopia/kopia/repo"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	repomocks "github.com/vmware-tanzu/velero/pkg/repository/udmrepo/kopialib/backend/mocks"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestOpen(t *testing.T) {
	var directRpo *repomocks.DirectRepository
	testCases := []struct {
		name           string
		repoOptions    udmrepo.RepoOptions
		returnRepo     *repomocks.DirectRepository
		repoOpen       func(context.Context, string, string, *repo.Options) (repo.Repository, error)
		newWriterError error
		expectedErr    string
		expected       *kopiaRepository
	}{
		{
			name:        "invalid config file",
			expectedErr: "invalid config file path",
		},
		{
			name: "config file doesn't exist",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
			},
			expectedErr: "repo config fake-file doesn't exist: stat fake-file: no such file or directory",
		},
		{
			name: "repo open fail, repo not exist",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, os.ErrNotExist
			},
			expectedErr: "error to open repo, repo doesn't exist: file does not exist",
		},
		{
			name: "repo open fail, other error",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, errors.New("fake-repo-open-error")
			},
			expectedErr: "error to open repo: fake-repo-open-error",
		},
		{
			name: "create repository writer fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo:     new(repomocks.DirectRepository),
			newWriterError: errors.New("fake-new-writer-error"),
			expectedErr:    "error to create repo writer: fake-new-writer-error",
		},
		{
			name: "create repository success",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				Description:    "fake-description",
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo: new(repomocks.DirectRepository),
			expected: &kopiaRepository{
				description: "fake-description",
				throttle: logThrottle{
					interval: defaultLogInterval,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := velerotest.NewLogger()

			service := kopiaRepoService{
				logger: logger,
			}

			if tc.repoOpen != nil {
				kopiaRepoOpen = tc.repoOpen
			}

			if tc.returnRepo != nil {
				directRpo = tc.returnRepo
			}

			if tc.returnRepo != nil {
				tc.returnRepo.On("NewWriter", mock.Anything, mock.Anything).Return(nil, nil, tc.newWriterError)
				tc.returnRepo.On("Close", mock.Anything).Return(nil)
			}

			repo, err := service.Open(context.Background(), tc.repoOptions)

			if repo != nil {
				require.Equal(t, tc.expected.description, repo.(*kopiaRepository).description)
				require.Equal(t, tc.expected.throttle.interval, repo.(*kopiaRepository).throttle.interval)
				require.Equal(t, repo.(*kopiaRepository).logger, logger)
			}

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestMaintain(t *testing.T) {
	var directRpo *repomocks.DirectRepository
	testCases := []struct {
		name               string
		repoOptions        udmrepo.RepoOptions
		returnRepo         *repomocks.DirectRepository
		returnRepoWriter   *repomocks.DirectRepositoryWriter
		repoOpen           func(context.Context, string, string, *repo.Options) (repo.Repository, error)
		newRepoWriterError error
		findManifestError  error
		expectedErr        string
	}{
		{
			name:        "invalid config file",
			expectedErr: "invalid config file path",
		},
		{
			name: "config file doesn't exist",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
			},
			expectedErr: "repo config fake-file doesn't exist: stat fake-file: no such file or directory",
		},
		{
			name: "repo open fail, repo not exist",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, os.ErrNotExist
			},
			expectedErr: "error to open repo, repo doesn't exist: file does not exist",
		},
		{
			name: "repo open fail, other error",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, errors.New("fake-repo-open-error")
			},
			expectedErr: "error to open repo: fake-repo-open-error",
		},
		{
			name: "write session fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo:         new(repomocks.DirectRepository),
			newRepoWriterError: errors.New("fake-new-direct-writer-error"),
			expectedErr:        "error to maintain repo: unable to create direct writer: fake-new-direct-writer-error",
		},
		{
			name: "maintain fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo:        new(repomocks.DirectRepository),
			returnRepoWriter:  new(repomocks.DirectRepositoryWriter),
			findManifestError: errors.New("fake-find-manifest-error"),
			expectedErr:       "error to maintain repo: error to run maintenance under mode auto: unable to get maintenance params: error looking for maintenance manifest: fake-find-manifest-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := velerotest.NewLogger()
			ctx := context.Background()

			service := kopiaRepoService{
				logger: logger,
			}

			if tc.repoOpen != nil {
				kopiaRepoOpen = tc.repoOpen
			}

			if tc.returnRepo != nil {
				directRpo = tc.returnRepo
			}

			if tc.returnRepo != nil {
				tc.returnRepo.On("NewDirectWriter", mock.Anything, mock.Anything).Return(ctx, tc.returnRepoWriter, tc.newRepoWriterError)
				tc.returnRepo.On("Close", mock.Anything).Return(nil)
			}

			if tc.returnRepoWriter != nil {
				tc.returnRepoWriter.On("DisableIndexRefresh").Return()
				tc.returnRepoWriter.On("AlsoLogToContentLog", mock.Anything).Return(nil)
				tc.returnRepoWriter.On("Close", mock.Anything).Return(nil)
				tc.returnRepoWriter.On("FindManifests", mock.Anything, mock.Anything).Return(nil, tc.findManifestError)
			}

			err := service.Maintain(ctx, tc.repoOptions)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestWriteInitParameters(t *testing.T) {
	var directRpo *repomocks.DirectRepository
	testCases := []struct {
		name               string
		repoOptions        udmrepo.RepoOptions
		returnRepo         *repomocks.DirectRepository
		returnRepoWriter   *repomocks.DirectRepositoryWriter
		repoOpen           func(context.Context, string, string, *repo.Options) (repo.Repository, error)
		newRepoWriterError error
		findManifestError  error
		expectedErr        string
	}{
		{
			name: "repo open fail, repo not exist",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, os.ErrNotExist
			},
			expectedErr: "error to open repo, repo doesn't exist: file does not exist",
		},
		{
			name: "repo open fail, other error",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return nil, errors.New("fake-repo-open-error")
			},
			expectedErr: "error to open repo: fake-repo-open-error",
		},
		{
			name: "write session fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo:         new(repomocks.DirectRepository),
			newRepoWriterError: errors.New("fake-new-writer-error"),
			expectedErr:        "error to init write repo parameters: unable to create writer: fake-new-writer-error",
		},
		{
			name: "set repo param fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "/tmp",
				GeneralOptions: map[string]string{},
			},
			repoOpen: func(context.Context, string, string, *repo.Options) (repo.Repository, error) {
				return directRpo, nil
			},
			returnRepo:        new(repomocks.DirectRepository),
			returnRepoWriter:  new(repomocks.DirectRepositoryWriter),
			findManifestError: errors.New("fake-find-manifest-error"),
			expectedErr:       "error to init write repo parameters: error to set maintenance params: error looking for maintenance manifest: fake-find-manifest-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := velerotest.NewLogger()
			ctx := context.Background()

			if tc.repoOpen != nil {
				kopiaRepoOpen = tc.repoOpen
			}

			if tc.returnRepo != nil {
				directRpo = tc.returnRepo
			}

			if tc.returnRepo != nil {
				tc.returnRepo.On("NewWriter", mock.Anything, mock.Anything).Return(ctx, tc.returnRepoWriter, tc.newRepoWriterError)
				tc.returnRepo.On("ClientOptions").Return(repo.ClientOptions{})
				tc.returnRepo.On("Close", mock.Anything).Return(nil)
			}

			if tc.returnRepoWriter != nil {
				tc.returnRepoWriter.On("Close", mock.Anything).Return(nil)
				tc.returnRepoWriter.On("FindManifests", mock.Anything, mock.Anything).Return(nil, tc.findManifestError)
			}

			err := writeInitParameters(ctx, tc.repoOptions, logger)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestShouldLog(t *testing.T) {
	testCases := []struct {
		name     string
		lastTime int64
		interval time.Duration
		retValue bool
	}{
		{
			name:     "first time",
			retValue: true,
		},
		{
			name:     "not run",
			lastTime: time.Now().Add(time.Hour).UnixNano(),
			interval: time.Second * 10,
		},
		{
			name:     "not first time, run",
			lastTime: time.Now().Add(-time.Hour).UnixNano(),
			interval: time.Second * 10,
			retValue: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lt := logThrottle{
				lastTime: tc.lastTime,
				interval: tc.interval,
			}

			before := lt.lastTime

			nw := time.Now()

			s := lt.shouldLog()

			require.Equal(t, s, tc.retValue)

			if s {
				require.GreaterOrEqual(t, lt.lastTime-nw.UnixNano(), lt.interval)
			} else {
				require.Equal(t, lt.lastTime, before)
			}
		})
	}
}
