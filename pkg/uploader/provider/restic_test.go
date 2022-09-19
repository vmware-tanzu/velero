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
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

func TestResticRunBackup(t *testing.T) {
	var rp resticProvider
	rp.log = logrus.New()
	updater := FakeBackupProgressUpdater{PodVolumeBackup: &velerov1api.PodVolumeBackup{}, Log: rp.log, Ctx: context.Background(), Cli: fake.NewFakeClientWithScheme(scheme.Scheme)}
	testCases := []struct {
		name              string
		hookBackupFunc    func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command
		hookRunBackupFunc func(backupCmd *restic.Command, log logrus.FieldLogger, updater uploader.ProgressUpdater) (string, string, error)
		errorHandleFunc   func(err error) bool
	}{
		{
			name: "wrong restic execute command",
			hookBackupFunc: func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command {
				return &restic.Command{Command: "date"}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "executable file not found in")
			},
		},
		{
			name: "wrong parsing json summary content",
			hookBackupFunc: func(repoIdentifier string, passwordFile string, path string, tags map[string]string) *restic.Command {
				return &restic.Command{Command: "version"}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "executable file not found in")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResticBackupCMDFunc = tc.hookBackupFunc
			_, _, err := rp.RunBackup(context.Background(), "var", nil, "", &updater)
			rp.log.Infof("test name %v error %v", tc.name, err)
			require.Equal(t, true, tc.errorHandleFunc(err))
		})
	}
}

func TestResticRunRestore(t *testing.T) {
	var rp resticProvider
	rp.log = logrus.New()
	updater := FakeBackupProgressUpdater{PodVolumeBackup: &velerov1api.PodVolumeBackup{}, Log: rp.log, Ctx: context.Background(), Cli: fake.NewFakeClientWithScheme(scheme.Scheme)}
	ResticRestoreCMDFunc = func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command {
		return &restic.Command{Args: []string{""}}
	}
	testCases := []struct {
		name                  string
		hookResticRestoreFunc func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command
		errorHandleFunc       func(err error) bool
	}{
		{
			name: "wrong restic execute command",
			hookResticRestoreFunc: func(repoIdentifier, passwordFile, snapshotID, target string) *restic.Command {
				return &restic.Command{Args: []string{"date"}}
			},
			errorHandleFunc: func(err error) bool {
				return strings.Contains(err.Error(), "executable file not found ")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResticRestoreCMDFunc = tc.hookResticRestoreFunc
			err := rp.RunRestore(context.Background(), "", "var", &updater)
			rp.log.Infof("test name %v error %v", tc.name, err)
			require.Equal(t, true, tc.errorHandleFunc(err))
		})
	}

}
