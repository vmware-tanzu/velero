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
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli/serverstatus"
)

func NewCommand(f client.Factory) *cobra.Command {
	var clientOnly bool
	timeout := 5 * time.Second

	c := &cobra.Command{
		Use:   "version",
		Short: "Print the velero version and associated image",
		Run: func(c *cobra.Command, args []string) {
			var mgr manager.Manager
			if !clientOnly {
				var err error
				mgr, err = f.KubebuilderManager()
				cmd.CheckError(err)

			}
			printVersion(os.Stdout, clientOnly, mgr, f.Namespace(), timeout)
		},
	}

	c.Flags().DurationVar(&timeout, "timeout", timeout, "maximum time to wait for server version to be reported. Default is 5 seconds.")
	c.Flags().BoolVar(&clientOnly, "client-only", clientOnly, "only get velero client version, not server version")

	return c
}

func printVersion(w io.Writer, clientOnly bool, mgr manager.Manager, namespace string, timeout time.Duration) {
	fmt.Fprintln(w, "Client:")
	fmt.Fprintf(w, "\tVersion: %s\n", buildinfo.Version)
	fmt.Fprintf(w, "\tGit commit: %s\n", buildinfo.FormattedGitSHA())

	if clientOnly {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	serverStatus, err := serverstatus.GetServerStatus(mgr, namespace, ctx)
	if err != nil {
		fmt.Fprintf(w, "<error getting server version: %s>\n", err)
		return
	}

	fmt.Fprintln(w, "Server:")
	fmt.Fprintf(w, "\tVersion: %s\n", serverStatus.Status.ServerVersion)
}
