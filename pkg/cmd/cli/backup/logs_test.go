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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	versionedmocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/mocks"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	velerov1mocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1/mocks"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewLogsCommand(t *testing.T) {
	backupName := "bk-logs-1"

	// create a factory
	f := &factorymocks.Factory{}

	backups := &velerov1mocks.BackupInterface{}
	veleroV1 := &velerov1mocks.VeleroV1Interface{}
	client := &versionedmocks.Interface{}
	bk := &velerov1api.Backup{}
	bkList := &velerov1api.BackupList{}
	kbclient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	backups.On("List", mock.Anything, mock.Anything).Return(bkList, nil)
	backups.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
	veleroV1.On("Backups", mock.Anything).Return(backups, nil)
	client.On("VeleroV1").Return(veleroV1, nil)
	f.On("Client").Return(client, nil)
	f.On("Namespace").Return(mock.Anything)
	f.On("KubebuilderClient").Return(kbclient, nil)

	c := NewLogsCommand(f)
	assert.Equal(t, "Get backup logs", c.Short)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		c.SetArgs([]string{backupName})
		e := c.Execute()
		assert.NoError(t, e)
		return
	}

	cmd := exec.Command(os.Args[0], []string{"-test.run=TestNewLogsCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	_, stderr, err := veleroexec.RunCommand(cmd)

	if err != nil {
		assert.Contains(t, stderr, fmt.Sprintf("Logs for backup \"%s\" are not available until it's finished processing", backupName))
		return
	}
	t.Fatalf("process ran with err %v, want backup delete successfully", err)
}
