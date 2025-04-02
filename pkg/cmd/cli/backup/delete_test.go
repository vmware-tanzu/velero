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
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestDeleteCommand(t *testing.T) {
	backup1 := "backup-name-1"
	backup2 := "backup-name-2"

	// create a factory
	f := &factorymocks.Factory{}

	client := velerotest.NewFakeControllerRuntimeClient(t)
	client.Create(context.Background(), builder.ForBackup(cmdtest.VeleroNameSpace, backup1).Result(), &controllerclient.CreateOptions{})
	client.Create(context.Background(), builder.ForBackup("default", backup2).Result(), &controllerclient.CreateOptions{})

	f.On("KubebuilderClient").Return(client, nil)
	f.On("Namespace").Return(cmdtest.VeleroNameSpace)

	// create command
	c := NewDeleteCommand(f, "velero backup delete")
	c.SetArgs([]string{backup1, backup2})
	require.Equal(t, "Delete backups", c.Short)

	o := cli.NewDeleteOptions("backup")
	flags := new(flag.FlagSet)
	o.BindFlags(flags)
	flags.Parse([]string{"--confirm"})

	args := []string{backup1, backup2}

	e := o.Complete(f, args)
	require.NoError(t, e)

	e = o.Validate(c, f, args)
	require.NoError(t, e)

	Run(o)

	e = c.Execute()
	require.NoError(t, e)

	if os.Getenv(cmdtest.CaptureFlag) == "1" {
		return
	}

	cmd := exec.Command(os.Args[0], []string{"-test.run=TestDeleteCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err := veleroexec.RunCommand(cmd)
	if err != nil {
		require.Contains(t, stdout, fmt.Sprintf("backups.velero.io \"%s\" not found.", backup2))
	}
}
