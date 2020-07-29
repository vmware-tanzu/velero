/*
Copyright 2017, 2019 the Velero contributors.

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

package version

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli/serverstatus"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
)

func NewCommand(f client.Factory) *cobra.Command {
	clientOnly := false
	serverStatusGetter := &serverstatus.DefaultServerStatusGetter{
		Namespace: f.Namespace(),
		Timeout:   5 * time.Second,
	}

	c := &cobra.Command{
		Use:   "version",
		Short: "Print the velero version and associated image",
		Run: func(c *cobra.Command, args []string) {
			var veleroClient velerov1client.ServerStatusRequestsGetter

			if !clientOnly {
				client, err := f.Client()
				cmd.CheckError(err)

				veleroClient = client.VeleroV1()
			}
			serverStatusGetter.Namespace = f.Namespace()
			printVersion(os.Stdout, clientOnly, veleroClient, serverStatusGetter)
		},
	}

	c.Flags().DurationVar(&serverStatusGetter.Timeout, "timeout", serverStatusGetter.Timeout, "Maximum time to wait for server version to be reported")
	c.Flags().BoolVar(&clientOnly, "client-only", clientOnly, "Only get velero client version, not server version")

	return c
}

func printVersion(w io.Writer, clientOnly bool, client velerov1client.ServerStatusRequestsGetter, serverStatusGetter serverstatus.ServerStatusGetter) {
	fmt.Fprintln(w, "Client:")
	fmt.Fprintf(w, "\tVersion: %s\n", buildinfo.Version)
	fmt.Fprintf(w, "\tGit commit: %s\n", buildinfo.FormattedGitSHA())

	if clientOnly {
		return
	}

	serverStatus, err := serverStatusGetter.GetServerStatus(client)
	if err != nil {
		fmt.Fprintf(w, "<error getting server version: %s>\n", err)
		return
	}

	fmt.Fprintln(w, "Server:")
	fmt.Fprintf(w, "\tVersion: %s\n", serverStatus.Status.ServerVersion)
}
