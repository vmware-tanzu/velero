/*
Copyright 2018 the Velero contributors.

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

package completion

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	kubectlcmd "github.com/vmware-tanzu/velero/third_party/kubernetes/pkg/kubectl/cmd"
)

func NewCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Output shell completion code for the specified shell (bash or zsh)",
		Long: `Generate shell completion code.

Auto completion supports both bash and zsh. Output is to STDOUT.

Load the velero completion code for bash into the current shell -
source <(velero completion bash)

Load the velero completion code for zsh into the current shell -
source <(velero completion zsh)
`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh"},
		Run: func(cmd *cobra.Command, args []string) {
			shell := args[0]
			switch shell {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				kubectlcmd.GenZshCompletion(os.Stdout, cmd.Root())
			default:
				fmt.Printf("Invalid shell specified, specify bash or zsh\n")
				os.Exit(1)
			}
		},
	}

	return c
}
