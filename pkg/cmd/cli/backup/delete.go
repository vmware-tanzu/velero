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

package backup

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

// NewDeleteCommand creates a new command that deletes a backup.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := cli.NewDeleteOptions("backup")

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAMES]", use),
		Short: "Delete backups",
		Example: `  # Delete a backup named "backup-1".
  velero backup delete backup-1

  # Delete a backup named "backup-1" without prompting for confirmation.
  velero backup delete backup-1 --confirm

  # Delete backups named "backup-1" and "backup-2".
  velero backup delete backup-1 backup-2

  # Delete all backups triggered by schedule "schedule-1".
  velero backup delete --selector velero.io/schedule-name=schedule-1
 
  # Delete all backups.
  velero backup delete --all`,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, f, args))
			cmd.CheckError(Run(o))
		},
	}

	o.BindFlags(c.Flags())

	return c
}

// Run performs the delete backup operation.
func Run(o *cli.DeleteOptions) error {
	if !o.Confirm && !cli.GetConfirmation() {
		// Don't do anything unless we get confirmation
		return nil
	}

	var (
		backups []*velerov1api.Backup
		errs    []error
	)

	// get the list of backups to delete
	switch {
	case len(o.Names) > 0:
		for _, name := range o.Names {
			backup, err := o.Client.VeleroV1().Backups(o.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, errors.WithStack(err))
				continue
			}

			backups = append(backups, backup)
		}
	default:
		selector := labels.Everything().String()
		if o.Selector.LabelSelector != nil {
			selector = o.Selector.String()
		}

		res, err := o.Client.VeleroV1().Backups(o.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return errors.WithStack(err)
		}
		for i := range res.Items {
			backups = append(backups, &res.Items[i])
		}
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return nil
	}

	// create a backup deletion request for each
	for _, b := range backups {
		deleteRequest := backup.NewDeleteBackupRequest(b.Name, string(b.UID))

		if _, err := o.Client.VeleroV1().DeleteBackupRequests(o.Namespace).Create(context.TODO(), deleteRequest, metav1.CreateOptions{}); err != nil {
			errs = append(errs, err)
			continue
		}

		fmt.Printf("Request to delete backup %q submitted successfully.\nThe backup will be fully deleted after all associated data (disk snapshots, backup files, restores) are removed.\n", b.Name)
	}

	return kubeerrs.NewAggregate(errs)
}
