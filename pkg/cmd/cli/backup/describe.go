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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/output"
	"github.com/heptio/velero/pkg/restic"
)

func NewDescribeCommand(f client.Factory, use string) *cobra.Command {
	var (
		listOptions        metav1.ListOptions
		details            bool
		insecureSkipVerify bool
	)

	c := &cobra.Command{
		Use:   use + " [NAME1] [NAME2] [NAME...]",
		Short: "Describe backups",
		Run: func(c *cobra.Command, args []string) {
			veleroClient, err := f.Client()
			cmd.CheckError(err)

			var backups *v1.BackupList
			if len(args) > 0 {
				backups = new(v1.BackupList)
				for _, name := range args {
					backup, err := veleroClient.VeleroV1().Backups(f.Namespace()).Get(name, metav1.GetOptions{})
					cmd.CheckError(err)
					backups.Items = append(backups.Items, *backup)
				}
			} else {
				backups, err = veleroClient.VeleroV1().Backups(f.Namespace()).List(listOptions)
				cmd.CheckError(err)
			}

			first := true
			for _, backup := range backups.Items {
				deleteRequestListOptions := pkgbackup.NewDeleteBackupRequestListOptions(backup.Name, string(backup.UID))
				deleteRequestList, err := veleroClient.VeleroV1().DeleteBackupRequests(f.Namespace()).List(deleteRequestListOptions)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting DeleteBackupRequests for backup %s: %v\n", backup.Name, err)
				}

				opts := restic.NewPodVolumeBackupListOptions(backup.Name)
				podVolumeBackupList, err := veleroClient.VeleroV1().PodVolumeBackups(f.Namespace()).List(opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting PodVolumeBackups for backup %s: %v\n", backup.Name, err)
				}

				s := output.DescribeBackup(&backup, deleteRequestList.Items, podVolumeBackupList.Items, details, veleroClient, insecureSkipVerify)
				if first {
					first = false
					fmt.Print(s)
				} else {
					fmt.Printf("\n\n%s", s)
				}
			}
			cmd.CheckError(err)
		},
	}

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "only show items matching this label selector")
	c.Flags().BoolVar(&details, "details", details, "display additional detail in the command output")
	c.Flags().BoolVar(&insecureSkipVerify, "insecureskipverify", insecureSkipVerify, "accept any TLS certificate presented by the storage service")

	return c
}
