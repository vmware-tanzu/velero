/*
Copyright 2017 the Heptio Ark contributors.

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

	"github.com/spf13/cobra"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/downloadrequest"
)

func NewLogsCommand(f client.Factory) *cobra.Command {
	timeout := time.Minute

	c := &cobra.Command{
		Use:   "logs BACKUP",
		Short: "Get backup logs",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			veleroClient, err := f.Client()
			cmd.CheckError(err)

			err = downloadrequest.Stream(veleroClient.VeleroV1(), f.Namespace(), args[0], v1.DownloadTargetKindBackupLog, os.Stdout, timeout)
			cmd.CheckError(err)
		},
	}

	c.Flags().DurationVar(&timeout, "timeout", timeout, "how long to wait to receive logs")

	return c
}
