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
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
	veleroflag "github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
)

func TestNewSetCommand(t *testing.T) {
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
	cmd := NewSetCommand(f, "")
	assert.Equal(t, "Set specific features for a backup storage location", cmd.Short)

	// create a SetOptions with full options set and then run this backup command
	cacert := "a/b/c/cert.ca"
	defaultBackupStorageLocation := false
	credential := veleroflag.NewMap()
	credential.Set("secret=a")

	flags := new(flag.FlagSet)
	o := NewSetOptions()
	o.BindFlags(flags)

	flags.Parse([]string{"--cacert", cacert})
	flags.Parse([]string{"--credential", credential.String()})
	flags.Parse([]string{"--DefaultBackupStorageLocation"})

	args := []string{"arg2"}
	o.Complete(args, f)
	e := o.Validate(cmd, args, f)
	assert.Equal(t, e, nil)

	e = o.Run(cmd, f)
	assert.Contains(t, e.Error(), fmt.Sprintf("Get \"%s/api?timeout=", clicmd.HOST))

	// verify all options are set as expected
	assert.Equal(t, cacert, o.CACertFile)
	assert.Equal(t, defaultBackupStorageLocation, o.DefaultBackupStorageLocation)
	assert.Equal(t, true, reflect.DeepEqual(credential, o.Credential))
}

func TestSetCommand_Execute(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
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
		c := NewSetCommand(f, "velero backup-location set")
		c.SetArgs([]string{"bsl-1"})
		c.Execute()
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestSetCommand_Execute"}...))
}
