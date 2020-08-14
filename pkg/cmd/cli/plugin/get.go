/*
Copyright 2020 the Velero contributors.

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

package plugin

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli/serverstatus"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
)

func NewGetCommand(f client.Factory, use string) *cobra.Command {
	// serverStatusGetter := &serverstatus.DefaultServerStatusGetter{
	// 	Timeout: 5 * time.Second,
	// }

	serverStatusGetter := &serverstatus.DefaultServerStatusGetter{
		Namespace: f.Namespace(),
		Timeout:   5 * time.Second,
	}

	c := &cobra.Command{
		Use:   use,
		Short: "Get information for all plugins on the velero server",
		Run: func(c *cobra.Command, args []string) {
			err := output.ValidateFlags(c)
			cmd.CheckError(err)
			fmt.Println("hererer")

			mgr, err := f.KubebuilderManager()
			// client, err := f.KubebuilderClient()
			cmd.CheckError(err)

			fmt.Println("hererer2222-eeee")

			// serverStatusList := new(velerov1api.ServerStatusRequestList)
			// serverStatusList, err := velero.ListBackupStorageLocations(r.StorageLocation.Client, r.StorageLocation.Ctx, req.Namespace)

			// var serverStatusList velerov1api.ServerStatusRequestList
			// if err := kbClient.List(context.Background(), &serverStatusList, &kbclient.ListOptions{
			// 	Namespace: f.Namespace(),
			// }); err != nil {
			// 	fmt.Fprintf(os.Stdout, "<error getting plugin information: %s>\n", err)
			// 	return
			// }

			// err = client.Get(context.Background(), kbclient.ObjectKey{
			// 	Namespace: f.Namespace(),
			// 	// Timeout:   5 * time.Second,
			// }, serverStatus)
			// if err != nil {
			// 	fmt.Fprintf(os.Stdout, "<error getting plugin information: %s>\n", err)
			// 	return
			// }

			// client, err := f.Client()
			// cmd.CheckError(err)

			// veleroClient := client.VeleroV1()

			serverStatus, err := serverStatusGetter.GetServerStatus(mgr)
			if err != nil {
				fmt.Fprintf(os.Stdout, "<error getting plugin information: %s>\n", err)
				return
			}

			_, err = output.PrintWithFormat(c, serverStatus)
			cmd.CheckError(err)
		},
	}

	c.Flags().DurationVar(&serverStatusGetter.Timeout, "timeout", serverStatusGetter.Timeout, "Maximum time to wait for plugin information to be reported.")
	output.BindFlagsSimple(c.Flags())

	return c
}
