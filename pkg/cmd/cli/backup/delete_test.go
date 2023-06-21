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

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	versionedmocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/mocks"
	velerov1mocks "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1/mocks"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestDeleteCommand(t *testing.T) {
	backupName := "backup-name-1"

	// create a factory
	f := &factorymocks.Factory{}

	deleteBackupRequest := &velerov1mocks.DeleteBackupRequestInterface{}
	backups := &velerov1mocks.BackupInterface{}
	veleroV1 := &velerov1mocks.VeleroV1Interface{}
	client := &versionedmocks.Interface{}
	bk := &velerov1api.Backup{}
	dbr := &velerov1api.DeleteBackupRequest{}
	backups.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(bk, nil)
	deleteBackupRequest.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(dbr, nil)
	veleroV1.On("DeleteBackupRequests", mock.Anything).Return(deleteBackupRequest, nil)
	veleroV1.On("Backups", mock.Anything).Return(backups, nil)
	client.On("VeleroV1").Return(veleroV1, nil)
	f.On("Client").Return(client, nil)
	f.On("Namespace").Return(mock.Anything)

	// create command
	c := NewDeleteCommand(f, "velero backup delete")
	c.SetArgs([]string{backupName})
	assert.Equal(t, "Delete backups", c.Short)

	o := cli.NewDeleteOptions("backup")
	flags := new(flag.FlagSet)
	o.BindFlags(flags)
	flags.Parse([]string{"--confirm"})

	args := []string{"bk1", "bk2"}

	bk.Name = backupName
	e := o.Complete(f, args)
	assert.Equal(t, e, nil)

	e = o.Validate(c, f, args)
	assert.Equal(t, e, nil)

	e = Run(o)
	assert.Equal(t, e, nil)

	e = c.Execute()
	assert.Equal(t, e, nil)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		return
	}

	cmd := exec.Command(os.Args[0], []string{"-test.run=TestDeleteCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err := veleroexec.RunCommand(cmd)

	if err == nil {
		assert.Contains(t, stdout, fmt.Sprintf("Request to delete backup \"%s\" submitted successfully.", backupName))
		return
	}
	t.Fatalf("process ran with err %v, want backups by get()", err)
}
