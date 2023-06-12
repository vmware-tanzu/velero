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
	"strconv"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
)

func TestNewDownloadCommand(t *testing.T) {
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
	cmd := NewDownloadCommand(f)
	assert.Equal(t, "Download all Kubernetes manifests for a backup", cmd.Short)

	// create a DownloadOptions with full options set and then run this backup command
	output := "path/to/download/bk.json"
	force := true
	timeout := "1m30s"
	insecureSkipTlsVerify := false
	cacert := "secret=YHJKKS"

	flags := new(flag.FlagSet)
	o := NewDownloadOptions()
	o.BindFlags(flags)

	flags.Parse([]string{"--output", output})
	flags.Parse([]string{"--force"})
	flags.Parse([]string{"--timeout", timeout})
	flags.Parse([]string{fmt.Sprintf("--insecure-skip-tls-verify=%s", strconv.FormatBool(insecureSkipTlsVerify))})
	flags.Parse([]string{"--cacert", cacert})

	backupName := "backup-1"
	args := []string{backupName, "arg2"}
	o.Complete(args)
	e := o.Validate(cmd, args, f)
	assert.Contains(t, e.Error(), fmt.Sprintf("Get \"%s/apis/velero.io/v1/namespaces/%s/backups/%s", clicmd.HOST, clicmd.VeleroNameSpace, backupName))

	// verify all options are set as expected
	assert.Equal(t, output, o.Output)
	assert.Equal(t, force, o.Force)
	assert.Equal(t, timeout, o.Timeout.String())
	assert.Equal(t, insecureSkipTlsVerify, o.InsecureSkipTLSVerify)
	assert.Equal(t, cacert, o.caCertFile)
}

func TestDownloadCommand_Execute(t *testing.T) {
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
		c := NewDownloadCommand(f)
		c.SetArgs([]string{"abc"})
		c.Execute()
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestDownloadCommand_Execute"}...))
}
