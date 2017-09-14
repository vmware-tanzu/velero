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
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/downloadrequest"
)

func NewLogsCommand(f client.Factory) *cobra.Command {
	timeout := time.Minute

	c := &cobra.Command{
		Use:   "logs BACKUP",
		Short: "Get backup logs",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				err := errors.New("backup name is required")
				cmd.CheckError(err)
			}

			arkClient, err := f.Client()
			cmd.CheckError(err)

			err = downloadrequest.Stream(arkClient.ArkV1(), args[0], v1.DownloadTargetKindBackupLog, os.Stdout, timeout)
			cmd.CheckError(err)
		},
	}

	c.Flags().DurationVar(&timeout, "timeout", timeout, "how long to wait to receive logs")

	return c
}
