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
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/client"
)

const testNamespaceDel = "velero"

func TestNewDeleteCommand(t *testing.T) {
	// create a config for factory
	baseName := "velero-bn"
	os.Setenv("VELERO_NAMESPACE", testNamespaceDel)
	config, err := client.LoadConfig()
	assert.Equal(t, err, nil)

	// create a factory
	f := client.NewFactory(baseName, config)
	cliFlags := new(flag.FlagSet)
	f.BindFlags(cliFlags)
	//host := "https://horse.org:4443"
	cliFlags.Parse([]string{"--kubeconfig", "kubeconfig", "--kubecontext", "federal-context"})

	// create command
	cmd := NewDeleteCommand(f, "aaa")
	assert.Equal(t, "Delete backups", cmd.Short)
	fmt.Println(cmd)
}
