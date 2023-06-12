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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	versionedmocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/mocks"
	velerov1mocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1/mocks"
)

func TestNewGetCommand(t *testing.T) {
	args := []string{"b1", "b2", "b3"}

	// create a factory
	f := &factorymocks.Factory{}

	backups := &velerov1mocks.BackupInterface{}
	veleroV1 := &velerov1mocks.VeleroV1Interface{}
	client := &versionedmocks.Interface{}
	bk := &velerov1api.Backup{}
	bkList := &velerov1api.BackupList{}

	backups.On("List", mock.Anything, mock.Anything).Return(bkList, nil)
	backups.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
	veleroV1.On("Backups", mock.Anything).Return(backups, nil)
	client.On("VeleroV1").Return(veleroV1, nil)
	f.On("Client").Return(client, nil)
	f.On("Namespace").Return(mock.Anything)

	// create command
	c := NewGetCommand(f, "velero backup get")
	assert.Equal(t, "Get backups", c.Short)

	c.SetArgs(args)
	e := c.Execute()
	assert.NoError(t, e)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		return
	}

	cmd := exec.Command(os.Args[0], []string{"-test.run=TestNewGetCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		output := strings.Split(stdout, "\n")
		i := 0
		for _, line := range output {
			if strings.Contains(line, "New") {
				i++
			}
		}
		assert.Equal(t, len(args), i)
		return
	}
	t.Fatalf("process ran with err %v, want backups by get()", err)
}
