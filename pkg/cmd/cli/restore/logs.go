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
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/downloadrequest"
)

func NewLogsCommand(f client.Factory) *cobra.Command {
	config, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error reading config file: %v\n", err)
	}

	timeout := time.Minute
	insecureSkipTLSVerify := false
	caCertFile := config.CACertFile()

	c := &cobra.Command{
		Use:   "logs RESTORE",
		Short: "Get restore logs",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			restoreName := args[0]

			veleroClient, err := f.Client()
			cmd.CheckError(err)

			kbClient, err := f.KubebuilderClient()
			cmd.CheckError(err)

			restore, err := veleroClient.VeleroV1().Restores(f.Namespace()).Get(context.TODO(), restoreName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				cmd.Exit("Restore %q does not exist.", restoreName)
			} else if err != nil {
				cmd.Exit("Error checking for restore %q: %v", restoreName, err)
			}

			switch restore.Status.Phase {
			case velerov1api.RestorePhaseCompleted, velerov1api.RestorePhaseFailed, velerov1api.RestorePhasePartiallyFailed, velerov1api.RestorePhaseWaitingForPluginOperations, velerov1api.RestorePhaseWaitingForPluginOperationsPartiallyFailed:
				// terminal and waiting for plugin operations phases, don't exit.
			default:
				cmd.Exit("Logs for restore %q are not available until it's finished processing. Please wait "+
					"until the restore has a phase of Completed or Failed and try again.", restoreName)
			}

			err = downloadrequest.Stream(context.Background(), kbClient, f.Namespace(), restoreName, velerov1api.DownloadTargetKindRestoreLog, os.Stdout, timeout, insecureSkipTLSVerify, caCertFile)
			cmd.CheckError(err)
		},
	}

	c.Flags().DurationVar(&timeout, "timeout", timeout, "How long to wait to receive logs.")
	c.Flags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", insecureSkipTLSVerify, "If true, the object store's TLS certificate will not be checked for validity. This is insecure and susceptible to man-in-the-middle attacks. Not recommended for production.")
	c.Flags().StringVar(&caCertFile, "cacert", caCertFile, "Path to a certificate bundle to use when verifying TLS connections.")

	return c
}
