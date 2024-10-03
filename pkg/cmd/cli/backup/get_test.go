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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func TestNewGetCommand(t *testing.T) {
	args := []string{"b1", "b2", "b3"}

	// create a factory
	f := &factorymocks.Factory{}

	client := velerotest.NewFakeControllerRuntimeClient(t)

	for _, backupName := range args {
		backup := builder.ForBackup(cmdtest.VeleroNameSpace, backupName).ObjectMeta(builder.WithLabels("abc", "abc")).Result()
		err := client.Create(context.Background(), backup, &kbclient.CreateOptions{})
		require.NoError(t, err)
	}

	f.On("KubebuilderClient").Return(client, nil)
	f.On("Namespace").Return(cmdtest.VeleroNameSpace)

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
	require.NoError(t, err)

	if err == nil {
		output := strings.Split(stdout, "\n")
		i := 0
		for _, line := range output {
			if strings.Contains(line, "New") {
				i++
			}
		}
		assert.Len(t, args, i)
	}

	d := NewGetCommand(f, "velero backup get")
	c.SetArgs([]string{"-l", "abc=abc"})
	e = d.Execute()
	assert.NoError(t, e)

	cmd = exec.Command(os.Args[0], []string{"-test.run=TestNewGetCommand"}...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", cmdtest.CaptureFlag))
	stdout, _, err = veleroexec.RunCommand(cmd)
	require.NoError(t, err)

	if err == nil {
		output := strings.Split(stdout, "\n")
		i := 0
		for _, line := range output {
			if strings.Contains(line, "New") {
				i++
			}
		}
		assert.Len(t, args, i)
	}
}
