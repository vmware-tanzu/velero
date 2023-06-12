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

	//"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/vmware-tanzu/velero/pkg/client"
	clicmd "github.com/vmware-tanzu/velero/pkg/cmd"
)

const testNamespaceForLog = "velero"

func TestNewLogsCommand(t *testing.T) {
	if os.Getenv(clicmd.TestExitFlag) == "1" {
		// create a config for factory
		baseName := "velero-bn"
		os.Setenv("VELERO_NAMESPACE", testNamespaceForLog)
		config, err := client.LoadConfig()
		assert.Equal(t, err, nil)

		// create a factory
		f := client.NewFactory(baseName, config)
		cliFlags := new(flag.FlagSet)
		f.BindFlags(cliFlags)
		cliFlags.Parse(clicmd.FactoryFlags)

		c := NewLogsCommand(f)
		assert.Equal(t, "Get backup logs", c.Short)
		fmt.Println(c)
		c.SetArgs([]string{"abc"})
		c.Execute()
		return
	}

	clicmd.TestProcessExit(t, exec.Command(os.Args[0], []string{"-test.run=TestNewLogsCommand"}...))
}
