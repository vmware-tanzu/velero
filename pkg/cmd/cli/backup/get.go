/*
Copyright 2017 the Velero contributors.

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
	"context"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
)

func NewGetCommand(f client.Factory, use string) *cobra.Command {
	var listOptions metav1.ListOptions

	c := &cobra.Command{
		Use:   use,
		Short: "Get backups",
		Run: func(c *cobra.Command, args []string) {
			err := output.ValidateFlags(c)
			cmd.CheckError(err)

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)

			backups := new(api.BackupList)
			if len(args) > 0 {
				for _, name := range args {
					backup := new(api.Backup)
					err := kbClient.Get(context.TODO(), kbclient.ObjectKey{Namespace: f.Namespace(), Name: name}, backup)
					cmd.CheckError(err)
					backups.Items = append(backups.Items, *backup)
				}
			} else {
				parsedSelector, err := labels.Parse(listOptions.LabelSelector)
				cmd.CheckError(err)
				err = kbClient.List(context.TODO(), backups, &kbclient.ListOptions{
					LabelSelector: parsedSelector,
					Namespace:     f.Namespace(),
				})
				cmd.CheckError(err)
			}

			_, err = output.PrintWithFormat(c, backups)
			cmd.CheckError(err)
		},
	}

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "Only show items matching this label selector")

	output.BindFlags(c.Flags())

	return c
}
