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

package backup

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/rest"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	"github.com/vmware-tanzu/velero/pkg/features"
	versionedmocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/mocks"
	velerov1mocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1/mocks"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewDescribeCommand(t *testing.T) {
	// create a factory
	f := &factorymocks.Factory{}

	backups := &velerov1mocks.BackupInterface{}
	veleroV1 := &velerov1mocks.VeleroV1Interface{}
	client := &versionedmocks.Interface{}
	clientConfig := rest.Config{}

	deleteBackupRequest := &velerov1mocks.DeleteBackupRequestInterface{}
	bk := &velerov1api.Backup{}
	bkList := &velerov1api.BackupList{}
	deleteBackupRequestList := &velerov1api.DeleteBackupRequestList{}
	podVolumeBackups := &velerov1mocks.PodVolumeBackupInterface{}
	podVolumeBackupList := &velerov1api.PodVolumeBackupList{}

	backupName := "bk-describe-1"
	bk.Name = backupName

	backups.On("List", mock.Anything, mock.Anything).Return(bkList, nil)
	backups.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
	veleroV1.On("Backups", mock.Anything).Return(backups, nil)
	deleteBackupRequest.On("List", mock.Anything, mock.Anything).Return(deleteBackupRequestList, nil)
	veleroV1.On("DeleteBackupRequests", mock.Anything).Return(deleteBackupRequest, nil)
	podVolumeBackups.On("List", mock.Anything, mock.Anything).Return(podVolumeBackupList, nil)
	veleroV1.On("PodVolumeBackups", mock.Anything, mock.Anything).Return(podVolumeBackups, nil)
	client.On("VeleroV1").Return(veleroV1, nil)
	f.On("ClientConfig").Return(&clientConfig, nil)
	f.On("Client").Return(client, nil)
	f.On("Namespace").Return(mock.Anything)
	f.On("KubebuilderClient").Return(nil, nil)

	// create command
	c := NewDescribeCommand(f, "velero backup describe")
	assert.Equal(t, "Describe backups", c.Short)

	features.NewFeatureFlagSet("EnableCSI")
	defer features.NewFeatureFlagSet()

	c.SetArgs([]string{"bk1"})
	e := c.Execute()
	assert.NoError(t, e)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		return
	}
	cmd := exec.Command(os.Args[0], []string{"-test.run=TestNewDescribeCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		assert.Contains(t, stdout, "Velero-Native Snapshots: <none included>")
		assert.Contains(t, stdout, fmt.Sprintf("Name:         %s", backupName))
		return
	}
	t.Fatalf("process ran with err %v, want backups by get()", err)
}
