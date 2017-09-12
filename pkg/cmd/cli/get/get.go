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

package get

import (
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd/cli/backup"
	"github.com/heptio/ark/pkg/cmd/cli/restore"
	"github.com/heptio/ark/pkg/cmd/cli/schedule"
)

func NewCommand(f client.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "get",
		Short: "Get ark resources",
		Long:  "Get ark resources",
	}

	backupCommand := backup.NewGetCommand(f, "backups")
	backupCommand.Aliases = []string{"backup"}

	scheduleCommand := schedule.NewGetCommand(f, "schedules")
	scheduleCommand.Aliases = []string{"schedule"}

	restoreCommand := restore.NewGetCommand(f, "restores")
	restoreCommand.Aliases = []string{"restore"}

	c.AddCommand(
		backupCommand,
		scheduleCommand,
		restoreCommand,
	)

	return c
}
