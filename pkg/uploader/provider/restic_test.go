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

package provider

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func TestResticRunBackup(t *testing.T) {
	testCases := []struct {
		name                        string
		nilUpdater                  bool
		parentSnapshot              string
		rp                          *resticProvider
		volMode                     uploader.PersistentVolumeMode
		hookBackupFunc              func(string, string, string, map[string]string) *restic.Command
		hookResticBackupFunc        func(*restic.Command, logrus.FieldLogger, uploader.ProgressUpdater) (string, string, error)
		hookResticGetSnapshotFunc   func(string, string, map[string]string) *restic.Command
		hookResticGetSnapshotIDFunc func(*restic.Command) (string, error)
		errorHandleFunc             func(err error) bool
	}{
		{
			name:       "nil uploader",
			rp:         &resticProvider{log: logrus.New()},
			nilUpdater: true,
			hookBackupFunc: func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "Need to initial backup progress updater first")
			},
		},
		{
			name: "wrong restic execute command",
			rp:   &resticProvider{log: logrus.New()},
			hookBackupFunc: func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "error running")
			},
		}, {
			name:           "has parent snapshot",
			rp:             &resticProvider{log: logrus.New()},
			parentSnapshot: "parentSnapshot",
			hookBackupFunc: func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			hookResticBackupFunc: func(*restic.Command, logrus.FieldLogger, uploader.ProgressUpdater) (string, string, error) {
				return "", "", nil
			},

			hookResticGetSnapshotIDFunc: func(*restic.Command) (string, error) { return "test-snapshot-id", nil },
			errorHandleFunc: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "has extra flags",
			rp:   &resticProvider{log: logrus.New(), extraFlags: []string{"testFlags"}},
			hookBackupFunc: func(string, string, string, map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			hookResticBackupFunc: func(*restic.Command, logrus.FieldLogger, uploader.ProgressUpdater) (string, string, error) {
				return "", "", nil
			},
			hookResticGetSnapshotIDFunc: func(*restic.Command) (string, error) { return "test-snapshot-id", nil },
			errorHandleFunc: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "failed to get snapshot id",
			rp:   &resticProvider{log: logrus.New(), extraFlags: []string{"testFlags"}},
			hookBackupFunc: func(string, string, string, map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			hookResticBackupFunc: func(*restic.Command, logrus.FieldLogger, uploader.ProgressUpdater) (string, string, error) {
				return "", "", nil
			},
			hookResticGetSnapshotIDFunc: func(*restic.Command) (string, error) {
				return "test-snapshot-id", errors.New("failed to get snapshot id")
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "failed to get snapshot id")
			},
		},
		{
			name:    "failed to use block mode",
			rp:      &resticProvider{log: logrus.New(), extraFlags: []string{"testFlags"}},
			volMode: uploader.PersistentVolumeBlock,
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "unable to support block mode")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			parentSnapshot := tc.parentSnapshot
			if tc.hookBackupFunc != nil {
				resticBackupCMDFunc = tc.hookBackupFunc
			}
			if tc.hookResticBackupFunc != nil {
				resticBackupFunc = tc.hookResticBackupFunc
			}
			if tc.hookResticGetSnapshotFunc != nil {
				resticGetSnapshotFunc = tc.hookResticGetSnapshotFunc
			}
			if tc.hookResticGetSnapshotIDFunc != nil {
				resticGetSnapshotIDFunc = tc.hookResticGetSnapshotIDFunc
			}
			if tc.volMode == "" {
				tc.volMode = uploader.PersistentVolumeFilesystem
			}
			if !tc.nilUpdater {
				updater := FakeBackupProgressUpdater{PodVolumeBackup: &velerov1api.PodVolumeBackup{}, Log: tc.rp.log, Ctx: context.Background(), Cli: fake.NewClientBuilder().WithScheme(util.VeleroScheme).Build()}
				_, _, err = tc.rp.RunBackup(context.Background(), "var", "", map[string]string{}, false, parentSnapshot, tc.volMode, map[string]string{}, &updater)
			} else {
				_, _, err = tc.rp.RunBackup(context.Background(), "var", "", map[string]string{}, false, parentSnapshot, tc.volMode, map[string]string{}, nil)
			}

			tc.rp.log.Infof("test name %v error %v", tc.name, err)
			require.True(t, tc.errorHandleFunc(err))
		})
	}
}

func TestResticRunRestore(t *testing.T) {
	resticRestoreCMDFunc = func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command {
		return &restic.Command{Args: []string{""}}
	}
	testCases := []struct {
		name                  string
		rp                    *resticProvider
		nilUpdater            bool
		hookResticRestoreFunc func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command
		errorHandleFunc       func(err error) bool
		volMode               uploader.PersistentVolumeMode
	}{
		{
			name:       "wrong restic execute command",
			rp:         &resticProvider{log: logrus.New()},
			nilUpdater: true,
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "Need to initial backup progress updater first")
			},
		},
		{
			name: "has extral flags",
			rp:   &resticProvider{log: logrus.New(), extraFlags: []string{"test-extra-flags"}},
			hookResticRestoreFunc: func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command {
				return &restic.Command{Args: []string{"date"}}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "error running command")
			},
		},
		{
			name: "wrong restic execute command",
			rp:   &resticProvider{log: logrus.New()},
			hookResticRestoreFunc: func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command {
				return &restic.Command{Args: []string{"date"}}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "error running command")
			},
		},
		{
			name: "error block volume mode",
			rp:   &resticProvider{log: logrus.New()},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "unable to support block mode")
			},
			volMode: uploader.PersistentVolumeBlock,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.volMode == "" {
				tc.volMode = uploader.PersistentVolumeFilesystem
			}
			resticRestoreCMDFunc = tc.hookResticRestoreFunc
			if tc.volMode == "" {
				tc.volMode = uploader.PersistentVolumeFilesystem
			}
			var err error
			if !tc.nilUpdater {
				updater := FakeBackupProgressUpdater{PodVolumeBackup: &velerov1api.PodVolumeBackup{}, Log: tc.rp.log, Ctx: context.Background(), Cli: fake.NewClientBuilder().WithScheme(util.VeleroScheme).Build()}
				err = tc.rp.RunRestore(context.Background(), "", "var", tc.volMode, map[string]string{}, &updater)
			} else {
				err = tc.rp.RunRestore(context.Background(), "", "var", tc.volMode, map[string]string{}, nil)
			}

			tc.rp.log.Infof("test name %v error %v", tc.name, err)
			require.True(t, tc.errorHandleFunc(err))
		})
	}
}

func TestClose(t *testing.T) {
	t.Run("Delete existing credentials file", func(t *testing.T) {
		// Create temporary files for the credentials and caCert
		credentialsFile, err := os.CreateTemp("", "credentialsFile")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(credentialsFile.Name())

		caCertFile, err := os.CreateTemp("", "caCertFile")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(caCertFile.Name())
		rp := &resticProvider{
			credentialsFile: credentialsFile.Name(),
			caCertFile:      caCertFile.Name(),
		}
		// Test deleting an existing credentials file
		err = rp.Close(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		_, err = os.Stat(rp.credentialsFile)
		if !os.IsNotExist(err) {
			t.Errorf("expected credentials file to be deleted, got error: %v", err)
		}
	})

	t.Run("Delete existing caCert file", func(t *testing.T) {
		// Create temporary files for the credentials and caCert
		caCertFile, err := os.CreateTemp("", "caCertFile")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(caCertFile.Name())
		rp := &resticProvider{
			credentialsFile: "",
			caCertFile:      "",
		}
		err = rp.Close(context.Background())
		// Test deleting an existing caCert file
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		_, err = os.Stat(rp.caCertFile)
		if !os.IsNotExist(err) {
			t.Errorf("expected caCert file to be deleted, got error: %v", err)
		}
	})
}

type MockCredentialGetter struct {
	mock.Mock
}

func (m *MockCredentialGetter) Path(selector *v1.SecretKeySelector) (string, error) {
	args := m.Called(selector)
	return args.Get(0).(string), args.Error(1)
}

func TestNewResticUploaderProvider(t *testing.T) {
	testCases := []struct {
		name                     string
		emptyBSL                 bool
		mockCredFunc             func(*MockCredentialGetter, *v1.SecretKeySelector)
		resticCmdEnvFunc         func(backupLocation *velerov1api.BackupStorageLocation, credentialFileStore credentials.FileStore) ([]string, error)
		resticTempCACertFileFunc func(caCert []byte, bsl string, fs filesystem.Interface) (string, error)
		checkFunc                func(provider Provider, err error)
	}{
		{
			name: "No error in creating temp credentials file",
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("temp-credentials", nil)
			},
			checkFunc: func(provider Provider, err error) {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			},
		}, {
			name: "Error in creating temp credentials file",
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("", errors.New("error creating temp credentials file"))
			},
			checkFunc: func(provider Provider, err error) {
				require.Error(t, err)
				assert.Nil(t, provider)
			},
		}, {
			name: "ObjectStorage with CACert present and creating CACert file failed",
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("temp-credentials", nil)
			},
			resticTempCACertFileFunc: func(caCert []byte, bsl string, fs filesystem.Interface) (string, error) {
				return "", errors.New("error writing CACert file")
			},
			checkFunc: func(provider Provider, err error) {
				require.Error(t, err)
				assert.Nil(t, provider)
			},
		}, {
			name: "Generating repository cmd failed",
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("temp-credentials", nil)
			},
			resticTempCACertFileFunc: func(caCert []byte, bsl string, fs filesystem.Interface) (string, error) {
				return "test-ca", nil
			},
			resticCmdEnvFunc: func(backupLocation *velerov1api.BackupStorageLocation, credentialFileStore credentials.FileStore) ([]string, error) {
				return nil, errors.New("error generating repository cmnd env")
			},
			checkFunc: func(provider Provider, err error) {
				require.Error(t, err)
				assert.Nil(t, provider)
			},
		}, {
			name: "New provider with not nil bsl",
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("temp-credentials", nil)
			},
			resticTempCACertFileFunc: func(caCert []byte, bsl string, fs filesystem.Interface) (string, error) {
				return "test-ca", nil
			},
			resticCmdEnvFunc: func(backupLocation *velerov1api.BackupStorageLocation, credentialFileStore credentials.FileStore) ([]string, error) {
				return nil, nil
			},
			checkFunc: func(provider Provider, err error) {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			},
		},
		{
			name:     "New provider with nil bsl",
			emptyBSL: true,
			mockCredFunc: func(credGetter *MockCredentialGetter, repoKeySelector *v1.SecretKeySelector) {
				credGetter.On("Path", repoKeySelector).Return("temp-credentials", nil)
			},
			resticTempCACertFileFunc: func(caCert []byte, bsl string, fs filesystem.Interface) (string, error) {
				return "test-ca", nil
			},
			resticCmdEnvFunc: func(backupLocation *velerov1api.BackupStorageLocation, credentialFileStore credentials.FileStore) ([]string, error) {
				return nil, nil
			},
			checkFunc: func(provider Provider, err error) {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoIdentifier := "my-repo"
			bsl := &velerov1api.BackupStorageLocation{}
			if !tc.emptyBSL {
				bsl = builder.ForBackupStorageLocation("test-ns", "test-name").CACert([]byte("my-cert")).Result()
			}
			credGetter := &credentials.CredentialGetter{}
			repoKeySelector := &v1.SecretKeySelector{}
			log := logrus.New()

			// Mock CredentialGetter
			mockCredGetter := &MockCredentialGetter{}
			credGetter.FromFile = mockCredGetter
			tc.mockCredFunc(mockCredGetter, repoKeySelector)
			if tc.resticCmdEnvFunc != nil {
				resticCmdEnvFunc = tc.resticCmdEnvFunc
			}
			if tc.resticTempCACertFileFunc != nil {
				resticTempCACertFileFunc = tc.resticTempCACertFileFunc
			}
			tc.checkFunc(NewResticUploaderProvider(repoIdentifier, bsl, credGetter, repoKeySelector, log))
		})
	}
}

func TestParseUploaderConfig(t *testing.T) {
	rp := &resticProvider{}

	testCases := []struct {
		name           string
		uploaderConfig map[string]string
		expectedFlags  []string
	}{
		{
			name: "SparseFilesEnabled",
			uploaderConfig: map[string]string{
				"WriteSparseFiles": "true",
			},
			expectedFlags: []string{"--sparse"},
		},
		{
			name: "SparseFilesDisabled",
			uploaderConfig: map[string]string{
				"writeSparseFiles": "false",
			},
			expectedFlags: []string{},
		},
		{
			name: "RestoreConcorrency",
			uploaderConfig: map[string]string{
				"Parallel": "5",
			},
			expectedFlags: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := rp.parseRestoreExtraFlags(testCase.uploaderConfig)
			if err != nil {
				t.Errorf("Test case %s failed with error: %v", testCase.name, err)
				return
			}

			if !reflect.DeepEqual(result, testCase.expectedFlags) {
				t.Errorf("Test case %s failed. Expected: %v, Got: %v", testCase.name, testCase.expectedFlags, result)
			}
		})
	}
}
