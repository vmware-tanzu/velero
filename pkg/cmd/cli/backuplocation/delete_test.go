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

package backuplocation

import (
	"os"
	"os/exec"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

func TestNewDeleteCommand(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
		// create a config for factory
		baseName := "velero-bn"
		os.Setenv("VELERO_NAMESPACE", cmd.VeleroNameSpace)
		config, err := client.LoadConfig()
		assert.Equal(t, err, nil)

		// create a factory
		f := client.NewFactory(baseName, config)
		cliFlags := new(flag.FlagSet)
		f.BindFlags(cliFlags)
		cliFlags.Parse(clicmd.FactoryFlags)

		// create command
		c := NewDeleteCommand(f, "velero backup-location delete")
		assert.Equal(t, "Delete backup storage locations", c.Short)

		o := cli.NewDeleteOptions("backup")
		flags := new(flag.FlagSet)
		o.BindFlags(flags)
		flags.Parse([]string{"--confirm"})

		args := []string{"bk-loc-1", "bk-loc-2"}
		e := o.Complete(f, args)
		assert.Equal(t, e, nil)
		e = o.Validate(c, f, args)
		assert.Equal(t, e, nil)

		Run(f, o)
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestNewDeleteCommand"}...))
}
