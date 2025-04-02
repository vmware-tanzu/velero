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
	"strconv"
	"testing"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestNewLogsCommand(t *testing.T) {
	t.Run("Flag test", func(t *testing.T) {
		l := NewLogsOptions()
		flags := new(flag.FlagSet)
		l.BindFlags(flags)

		timeout := "1m0s"
		insecureSkipTLSVerify := "true"
		caCertFile := "testing"

		flags.Parse([]string{"--timeout", timeout})
		flags.Parse([]string{"--insecure-skip-tls-verify", insecureSkipTLSVerify})
		flags.Parse([]string{"--cacert", caCertFile})

		require.Equal(t, timeout, l.Timeout.String())
		require.Equal(t, insecureSkipTLSVerify, strconv.FormatBool(l.InsecureSkipTLSVerify))
		require.Equal(t, caCertFile, l.CaCertFile)
	})

	t.Run("Backup not complete test", func(t *testing.T) {
		backupName := "bk-logs-1"

		// create a factory
		f := &factorymocks.Factory{}

		kbClient := velerotest.NewFakeControllerRuntimeClient(t)
		backup := builder.ForBackup(cmdtest.VeleroNameSpace, backupName).Result()
		err := kbClient.Create(context.Background(), backup, &kbclient.CreateOptions{})
		require.NoError(t, err)

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)
		f.On("KubebuilderClient").Return(kbClient, nil)

		c := NewLogsCommand(f)
		assert.Equal(t, "Get backup logs", c.Short)

		l := NewLogsOptions()
		flags := new(flag.FlagSet)
		l.BindFlags(flags)
		err = l.Complete([]string{backupName}, f)
		require.NoError(t, err)

		err = l.Run(c, f)
		require.Error(t, err)
		require.ErrorContains(t, err, fmt.Sprintf("logs for backup \"%s\" are not available until it's finished processing", backupName))
	})

	t.Run("Backup not exist test", func(t *testing.T) {
		backupName := "not-exist"
		// create a factory
		f := &factorymocks.Factory{}

		kbClient := velerotest.NewFakeControllerRuntimeClient(t)

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)
		f.On("KubebuilderClient").Return(kbClient, nil)

		c := NewLogsCommand(f)
		assert.Equal(t, "Get backup logs", c.Short)

		l := NewLogsOptions()
		flags := new(flag.FlagSet)
		l.BindFlags(flags)
		err := l.Complete([]string{backupName}, f)
		require.NoError(t, err)

		err = l.Run(c, f)
		require.Error(t, err)

		require.Equal(t, fmt.Sprintf("backup \"%s\" does not exist", backupName), err.Error())

		c.Execute()
	})

	t.Run("Normal backup log test", func(t *testing.T) {
		backupName := "bk-logs-1"

		// create a factory
		f := &factorymocks.Factory{}

		kbClient := velerotest.NewFakeControllerRuntimeClient(t)
		backup := builder.ForBackup(cmdtest.VeleroNameSpace, backupName).Phase(velerov1api.BackupPhaseCompleted).Result()
		err := kbClient.Create(context.Background(), backup, &kbclient.CreateOptions{})
		require.NoError(t, err)

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)
		f.On("KubebuilderClient").Return(kbClient, nil)

		c := NewLogsCommand(f)
		assert.Equal(t, "Get backup logs", c.Short)

		l := NewLogsOptions()
		flags := new(flag.FlagSet)
		l.BindFlags(flags)
		err = l.Complete([]string{backupName}, f)
		require.NoError(t, err)

		timeout := time.After(3 * time.Second)
		done := make(chan bool)
		go func() {
			err = l.Run(c, f)
			require.Error(t, err)
		}()

		select {
		case <-timeout:
			t.Skip("Test didn't finish in time, because BSL is not in Available state.")
		case <-done:
		}
	})

	t.Run("Invalid client test", func(t *testing.T) {
		// create a factory
		f := &factorymocks.Factory{}

		kbClient := velerotest.NewFakeControllerRuntimeClient(t)

		f.On("Namespace").Return(cmdtest.VeleroNameSpace)

		c := NewLogsCommand(f)
		assert.Equal(t, "Get backup logs", c.Short)

		l := NewLogsOptions()
		flags := new(flag.FlagSet)
		l.BindFlags(flags)

		f.On("KubebuilderClient").Return(kbClient, fmt.Errorf("test error"))
		err := l.Complete([]string{""}, f)
		require.Equal(t, "test error", err.Error())
	})
}
