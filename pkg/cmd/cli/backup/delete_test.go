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
	"os"
	"os/exec"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

func TestNewDeleteCommand(t *testing.T) {
	// create a config for factory
	baseName := "velero-bn"
	os.Setenv("VELERO_NAMESPACE", clicmd.VeleroNameSpace)
	config, err := client.LoadConfig()
	assert.Equal(t, err, nil)

	// create a factory
	f := client.NewFactory(baseName, config)
	cliFlags := new(flag.FlagSet)
	f.BindFlags(cliFlags)
	cliFlags.Parse(clicmd.FactoryFlags)

	// create command
	c := NewDeleteCommand(f, "velero backup delete")
	assert.Equal(t, "Delete backups", c.Short)

	o := cli.NewDeleteOptions("backup")
	flags := new(flag.FlagSet)
	o.BindFlags(flags)
	flags.Parse([]string{"--confirm"})

	args := []string{"bk1", "bk2"}
	e := o.Complete(f, args)
	assert.Equal(t, e, nil)
	e = o.Validate(c, f, args)
	assert.Equal(t, e, nil)

	e = Run(o)
	assert.Equal(t, e, nil)
}

func TestDeleteCommand_Execute(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
		// create a config for factory
		config, err := client.LoadConfig()
		assert.Equal(t, err, nil)

		// create a factory
		f := client.NewFactory("velero-bn", config)

		// create command
		cmd := NewDeleteCommand(f, "")
		cmd.Execute()
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestDeleteCommand_Execute"}...))
}
