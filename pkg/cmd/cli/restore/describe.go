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

package restore

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
	"github.com/vmware-tanzu/velero/pkg/label"
)

func NewDescribeCommand(f client.Factory, use string) *cobra.Command {
	var (
		listOptions           metav1.ListOptions
		details               bool
		insecureSkipTLSVerify bool
	)

	config, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error reading config file: %v\n", err)
	}
	caCertFile := config.CACertFile()

	c := &cobra.Command{
		Use:   use + " [NAME1] [NAME2] [NAME...]",
		Short: "Describe restores",
		Run: func(c *cobra.Command, args []string) {
			veleroClient, err := f.Client()
			cmd.CheckError(err)

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)

			var restores *velerov1api.RestoreList
			if len(args) > 0 {
				restores = new(velerov1api.RestoreList)
				for _, name := range args {
					restore, err := veleroClient.VeleroV1().Restores(f.Namespace()).Get(context.TODO(), name, metav1.GetOptions{})
					cmd.CheckError(err)
					restores.Items = append(restores.Items, *restore)
				}
			} else {
				restores, err = veleroClient.VeleroV1().Restores(f.Namespace()).List(context.TODO(), listOptions)
				cmd.CheckError(err)
			}

			first := true
			for i, restore := range restores.Items {
				opts := newPodVolumeRestoreListOptions(restore.Name)
				podvolumeRestoreList, err := veleroClient.VeleroV1().PodVolumeRestores(f.Namespace()).List(context.TODO(), opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting PodVolumeRestores for restore %s: %v\n", restore.Name, err)
				}

				s := output.DescribeRestore(context.Background(), kbClient, &restores.Items[i], podvolumeRestoreList.Items, details, veleroClient, insecureSkipTLSVerify, caCertFile)
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

	c.Flags().StringVarP(&listOptions.LabelSelector, "selector", "l", listOptions.LabelSelector, "Only show items matching this label selector.")
	c.Flags().BoolVar(&details, "details", details, "Display additional detail in the command output.")
	c.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", insecureSkipTLSVerify, "If true, the object store's TLS certificate will not be checked for validity. This is insecure and susceptible to man-in-the-middle attacks. Not recommended for production.")
	c.Flags().StringVar(&caCertFile, "cacert", caCertFile, "Path to a certificate bundle to use when verifying TLS connections.")

	return c
}

// newPodVolumeRestoreListOptions creates a ListOptions with a label selector configured to
// find PodVolumeRestores for the restore identified by name.
func newPodVolumeRestoreListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", velerov1api.RestoreNameLabel, label.GetValidName(name)),
	}
}
