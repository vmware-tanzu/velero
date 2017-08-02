/*
Copyright 2017 Heptio Inc.

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

	"github.com/spf13/cobra"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
)

func NewDeleteCommand(f client.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a backup",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.Usage()
				os.Exit(1)
			}

			arkClient, err := f.Client()
			cmd.CheckError(err)

			backupName := args[0]

			err = arkClient.ArkV1().Backups(api.DefaultNamespace).Delete(backupName, nil)
			cmd.CheckError(err)

			fmt.Printf("Backup %q deleted\n", backupName)
		},
	}

	return c
}
