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
	"github.com/kopia/kopia/repo/manifest"
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
		name                 string
		repoOptions          udmrepo.RepoOptions
		returnRepo           *repomocks.DirectRepository
		returnRepoWriter     *repomocks.DirectRepositoryWriter
		repoOpen             func(context.Context, string, string, *repo.Options) (repo.Repository, error)
		newRepoWriterError   error
		replaceManifestError error
		expectedErr          string
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
			returnRepo:           new(repomocks.DirectRepository),
			returnRepoWriter:     new(repomocks.DirectRepositoryWriter),
			replaceManifestError: errors.New("fake-replace-manifest-error"),
			expectedErr:          "error to init write repo parameters: error to set maintenance params: put manifest: fake-replace-manifest-error",
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
				tc.returnRepoWriter.On("ReplaceManifests", mock.Anything, mock.Anything, mock.Anything).Return(manifest.ID(""), tc.replaceManifestError)
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

func TestOpenObject(t *testing.T) {
	testCases := []struct {
		name        string
		rawRepo     *repomocks.DirectRepository
		objectID    string
		retErr      error
		expectedErr string
	}{
		{
			name:        "raw repo is nil",
			expectedErr: "repo is closed or not open",
		},
		{
			name:        "objectID is invalid",
			rawRepo:     repomocks.NewDirectRepository(t),
			objectID:    "fake-id",
			expectedErr: "error to parse object ID from fake-id: malformed content ID: \"fake-id\": invalid content prefix",
		},
		{
			name:        "raw open fail",
			rawRepo:     repomocks.NewDirectRepository(t),
			retErr:      errors.New("fake-open-error"),
			expectedErr: "error to open object: fake-open-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawRepo != nil {
				if tc.retErr != nil {
					tc.rawRepo.On("OpenObject", mock.Anything, mock.Anything).Return(nil, tc.retErr)
				}

				kr.rawRepo = tc.rawRepo
			}

			_, err := kr.OpenObject(context.Background(), udmrepo.ID(tc.objectID))

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestGetManifest(t *testing.T) {
	testCases := []struct {
		name        string
		rawRepo     *repomocks.DirectRepository
		retErr      error
		expectedErr string
	}{
		{
			name:        "raw repo is nil",
			expectedErr: "repo is closed or not open",
		},
		{
			name:        "raw get fail",
			rawRepo:     repomocks.NewDirectRepository(t),
			retErr:      errors.New("fake-get-error"),
			expectedErr: "error to get manifest: fake-get-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawRepo != nil {
				if tc.retErr != nil {
					tc.rawRepo.On("GetManifest", mock.Anything, mock.Anything, mock.Anything).Return(nil, tc.retErr)
				}

				kr.rawRepo = tc.rawRepo
			}

			err := kr.GetManifest(context.Background(), udmrepo.ID(""), &udmrepo.RepoManifest{})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestFindManifests(t *testing.T) {
	testCases := []struct {
		name        string
		rawRepo     *repomocks.DirectRepository
		retErr      error
		expectedErr string
	}{
		{
			name:        "raw repo is nil",
			expectedErr: "repo is closed or not open",
		},
		{
			name:        "raw find fail",
			rawRepo:     repomocks.NewDirectRepository(t),
			retErr:      errors.New("fake-find-error"),
			expectedErr: "error to find manifests: fake-find-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawRepo != nil {
				tc.rawRepo.On("FindManifests", mock.Anything, mock.Anything).Return(nil, tc.retErr)
				kr.rawRepo = tc.rawRepo
			}

			_, err := kr.FindManifests(context.Background(), udmrepo.ManifestFilter{})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestClose(t *testing.T) {
	testCases := []struct {
		name            string
		rawRepo         *repomocks.DirectRepository
		rawWriter       *repomocks.DirectRepositoryWriter
		rawRepoRetErr   error
		rawWriterRetErr error
		expectedErr     string
	}{
		{
			name: "both nil",
		},
		{
			name:      "writer is not nil",
			rawWriter: repomocks.NewDirectRepositoryWriter(t),
		},
		{
			name:    "repo is not nil",
			rawRepo: repomocks.NewDirectRepository(t),
		},
		{
			name:            "writer close error",
			rawWriter:       repomocks.NewDirectRepositoryWriter(t),
			rawWriterRetErr: errors.New("fake-writer-close-error"),
			expectedErr:     "error to close repo writer: fake-writer-close-error",
		},
		{
			name:          "repo is not nil",
			rawRepo:       repomocks.NewDirectRepository(t),
			rawRepoRetErr: errors.New("fake-repo-close-error"),
			expectedErr:   "error to close repo: fake-repo-close-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawRepo != nil {
				tc.rawRepo.On("Close", mock.Anything).Return(tc.rawRepoRetErr)
				kr.rawRepo = tc.rawRepo
			}

			if tc.rawWriter != nil {
				tc.rawWriter.On("Close", mock.Anything).Return(tc.rawWriterRetErr)
				kr.rawWriter = tc.rawWriter
			}

			err := kr.Close(context.Background())

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestPutManifest(t *testing.T) {
	testCases := []struct {
		name            string
		rawWriter       *repomocks.DirectRepositoryWriter
		rawWriterRetErr error
		expectedErr     string
	}{
		{
			name:        "raw writer is nil",
			expectedErr: "repo writer is closed or not open",
		},
		{
			name:            "raw put fail",
			rawWriter:       repomocks.NewDirectRepositoryWriter(t),
			rawWriterRetErr: errors.New("fake-writer-put-error"),
			expectedErr:     "error to put manifest: fake-writer-put-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawWriter != nil {
				tc.rawWriter.On("PutManifest", mock.Anything, mock.Anything, mock.Anything).Return(manifest.ID(""), tc.rawWriterRetErr)
				kr.rawWriter = tc.rawWriter
			}

			_, err := kr.PutManifest(context.Background(), udmrepo.RepoManifest{
				Metadata: &udmrepo.ManifestEntryMetadata{},
			})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestDeleteManifest(t *testing.T) {
	testCases := []struct {
		name            string
		rawWriter       *repomocks.DirectRepositoryWriter
		rawWriterRetErr error
		expectedErr     string
	}{
		{
			name:        "raw writer is nil",
			expectedErr: "repo writer is closed or not open",
		},
		{
			name:            "raw delete fail",
			rawWriter:       repomocks.NewDirectRepositoryWriter(t),
			rawWriterRetErr: errors.New("fake-writer-delete-error"),
			expectedErr:     "error to delete manifest: fake-writer-delete-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawWriter != nil {
				tc.rawWriter.On("DeleteManifest", mock.Anything, mock.Anything).Return(tc.rawWriterRetErr)
				kr.rawWriter = tc.rawWriter
			}

			err := kr.DeleteManifest(context.Background(), udmrepo.ID(""))

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestFlush(t *testing.T) {
	testCases := []struct {
		name            string
		rawWriter       *repomocks.DirectRepositoryWriter
		rawWriterRetErr error
		expectedErr     string
	}{
		{
			name:        "raw writer is nil",
			expectedErr: "repo writer is closed or not open",
		},
		{
			name:            "raw flush fail",
			rawWriter:       repomocks.NewDirectRepositoryWriter(t),
			rawWriterRetErr: errors.New("fake-writer-flush-error"),
			expectedErr:     "error to flush repo: fake-writer-flush-error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kr := &kopiaRepository{}

			if tc.rawWriter != nil {
				tc.rawWriter.On("Flush", mock.Anything).Return(tc.rawWriterRetErr)
				kr.rawWriter = tc.rawWriter
			}

			err := kr.Flush(context.Background())

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
