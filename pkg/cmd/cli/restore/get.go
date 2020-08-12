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

package restore

import (
	"context"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
)

func NewGetCommand(f client.Factory, use string) *cobra.Command {
	var listOptions metav1.ListOptions

	c := &cobra.Command{
		Use:   use,
		Short: "Get restores",
		Run: func(c *cobra.Command, args []string) {
			err := output.ValidateFlags(c)
			cmd.CheckError(err)

			veleroClient, err := f.Client()
			cmd.CheckError(err)

			var restores *api.RestoreList
			if len(args) > 0 {
				restores = new(api.RestoreList)
				for _, name := range args {
					restore, err := veleroClient.VeleroV1().Restores(f.Namespace()).Get(context.TODO(), name, metav1.GetOptions{})
					cmd.CheckError(err)
					restores.Items = append(restores.Items, *restore)
				}
			} else {
				restores, err = veleroClient.VeleroV1().Restores(f.Namespace()).List(context.TODO(), listOptions)
				cmd.CheckError(err)
			}

			if printed, err := output.PrintWithFormat(c, restores); printed || err != nil {
				cmd.CheckError(err)
				return
			}

			_, err = output.PrintWithFormat(c, restores)
			cmd.CheckError(err)
		},
	}

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "Only show items matching this label selector.")

	output.BindFlags(c.Flags())

	return c
}
