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

package backuplocation

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

// NewDeleteCommand creates and returns a new cobra command for deleting backup-locations.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := cli.NewDeleteOptions("backup-location")

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAMES]", use),
		Short: "Delete backup storage locations",
		Example: `  # Delete a backup storage location named "backup-location-1".
  velero backup-location delete backup-location-1

  # Delete a backup storage location named "backup-location-1" without prompting for confirmation.
  velero backup-location delete backup-location-1 --confirm

  # Delete backup storage locations named "backup-location-1" and "backup-location-2".
  velero backup-location delete backup-location-1 backup-location-2
		
  # Delete all backup storage locations labelled with "foo=bar".
  velero backup-location delete --selector foo=bar

  # Delete all backup storage locations.
  velero backup-location delete --all`,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, f, args))
			cmd.CheckError(Run(f, o))
		},
	}

	o.BindFlags(c.Flags())
	return c
}

// Run performs the delete backup-location operation.
func Run(f client.Factory, o *cli.DeleteOptions) error {
	if !o.Confirm && !cli.GetConfirmation() {
		// Don't do anything unless we get confirmation
		return nil
	}

	kbClient, err := f.KubebuilderClient()
	cmd.CheckError(err)

	locations := new(velerov1api.BackupStorageLocationList)
	var errs []error
	switch {
	case len(o.Names) > 0:
		for _, name := range o.Names {
			location := &velerov1api.BackupStorageLocation{}
			err = kbClient.Get(context.Background(), kbclient.ObjectKey{
				Namespace: f.Namespace(),
				Name:      name,
			}, location)
			if err != nil {
				errs = append(errs, errors.WithStack(err))
				continue
			}

			locations.Items = append(locations.Items, *location)
		}
	default:
		selector := labels.Everything().String()
		if o.Selector.LabelSelector != nil {
			selector = o.Selector.String()
		}

		err := kbClient.List(context.Background(), locations, &kbclient.ListOptions{
			Namespace: f.Namespace(),
			Raw:       &metav1.ListOptions{LabelSelector: selector},
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if len(locations.Items) == 0 {
		fmt.Println("No backup-locations found")
		return nil
	}

	// Create a backup-location deletion request for each
	for _, location := range locations.Items {
		if err := kbClient.Delete(context.Background(), &location, &kbclient.DeleteOptions{}); err != nil {
			errs = append(errs, errors.WithStack(err))
			continue
		}
		fmt.Printf("Backup storage location %q deleted successfully.\n", location.Name)
	}

	return kubeerrs.NewAggregate(errs)
}
