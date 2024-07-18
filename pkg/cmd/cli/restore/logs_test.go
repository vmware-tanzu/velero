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

package restore

import (
	"os"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	factorymocks "github.com/vmware-tanzu/velero/pkg/client/mocks"
	cmdtest "github.com/vmware-tanzu/velero/pkg/cmd/test"
)

func TestNewLogsCommand(t *testing.T) {
	t.Run("Flag test", func(t *testing.T) {
		// create a factory
		f := &factorymocks.Factory{}

		c := NewLogsCommand(f)
		require.Equal(t, "Get restore logs", c.Short)
		flags := new(flag.FlagSet)

		timeout := "1m0s"
		insecureSkipTLSVerify := "true"
		caCertFile := "testing"

		flags.Parse([]string{"--timeout", timeout})
		flags.Parse([]string{"--insecure-skip-tls-verify", insecureSkipTLSVerify})
		flags.Parse([]string{"--cacert", caCertFile})

		if os.Getenv(cmdtest.CaptureFlag) == "1" {
			c.SetArgs([]string{"test"})
			e := c.Execute()
			require.NoError(t, e)
			return
		}
	})
}
