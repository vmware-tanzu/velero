/*
Copyright the Velero contributors.

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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotv1client "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/label"
)

func NewDescribeCommand(f client.Factory, use string) *cobra.Command {
	var (
		listOptions           metav1.ListOptions
		details               bool
		insecureSkipTLSVerify bool
		outputFormat          = "plaintext"
	)

	config, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error reading config file: %v\n", err)
	}
	caCertFile := config.CACertFile()

	c := &cobra.Command{
		Use:   use + " [NAME1] [NAME2] [NAME...]",
		Short: "Describe backups",
		Run: func(c *cobra.Command, args []string) {
			veleroClient, err := f.Client()
			cmd.CheckError(err)

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)

			if outputFormat != "plaintext" && outputFormat != "json" {
				cmd.CheckError(fmt.Errorf("Invalid output format '%s'. Valid value are 'plaintext, json'", outputFormat))
			}

			var backups *velerov1api.BackupList
			if len(args) > 0 {
				backups = new(velerov1api.BackupList)
				for _, name := range args {
					backup, err := veleroClient.VeleroV1().Backups(f.Namespace()).Get(context.TODO(), name, metav1.GetOptions{})
					cmd.CheckError(err)
					backups.Items = append(backups.Items, *backup)
				}
			} else {
				backups, err = veleroClient.VeleroV1().Backups(f.Namespace()).List(context.TODO(), listOptions)
				cmd.CheckError(err)
			}

			first := true
			for i, backup := range backups.Items {
				deleteRequestListOptions := pkgbackup.NewDeleteBackupRequestListOptions(backup.Name, string(backup.UID))
				deleteRequestList, err := veleroClient.VeleroV1().DeleteBackupRequests(f.Namespace()).List(context.TODO(), deleteRequestListOptions)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting DeleteBackupRequests for backup %s: %v\n", backup.Name, err)
				}

				opts := label.NewListOptionsForBackup(backup.Name)
				podVolumeBackupList, err := veleroClient.VeleroV1().PodVolumeBackups(f.Namespace()).List(context.TODO(), opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting PodVolumeBackups for backup %s: %v\n", backup.Name, err)
				}

				var csiClient *snapshotv1client.Clientset
				// declare vscList up here since it may be empty and we'll pass the empty Items field into DescribeBackup
				vscList := new(snapshotv1api.VolumeSnapshotContentList)
				if features.IsEnabled(velerov1api.CSIFeatureFlag) {
					clientConfig, err := f.ClientConfig()
					cmd.CheckError(err)

					csiClient, err = snapshotv1client.NewForConfig(clientConfig)
					cmd.CheckError(err)

					vscList, err = csiClient.SnapshotV1().VolumeSnapshotContents().List(context.TODO(), opts)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error getting VolumeSnapshotContent objects for backup %s: %v\n", backup.Name, err)
					}
				}

				// structured output only applies to a single backup in case of OOM
				// To describe the list of backups in structured format, users could iterate over the list and describe backup one after another.
				if len(backups.Items) == 1 && outputFormat != "plaintext" {
					s := output.DescribeBackupInSF(context.Background(), kbClient, &backups.Items[i], deleteRequestList.Items, podVolumeBackupList.Items, vscList.Items, details, veleroClient, insecureSkipTLSVerify, caCertFile, outputFormat)
					fmt.Print(s)
				} else {
					s := output.DescribeBackup(context.Background(), kbClient, &backups.Items[i], deleteRequestList.Items, podVolumeBackupList.Items, vscList.Items, details, veleroClient, insecureSkipTLSVerify, caCertFile)
					if first {
						first = false
						fmt.Print(s)
					} else {
						fmt.Printf("\n\n%s", s)
					}
				}

			}
			cmd.CheckError(err)
		},
	}

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "Only show items matching this label selector.")
	c.Flags().BoolVar(&details, "details", details, "Display additional detail in the command output.")
	c.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", insecureSkipTLSVerify, "If true, the object store's TLS certificate will not be checked for validity. This is insecure and susceptible to man-in-the-middle attacks. Not recommended for production.")
	c.Flags().StringVar(&caCertFile, "cacert", caCertFile, "Path to a certificate bundle to use when verifying TLS connections.")
	c.Flags().StringVarP(&outputFormat, "output", "o", outputFormat, "Output display format. Valid formats are 'plaintext, json'. 'json' only applies to a single backup")

	return c
}
