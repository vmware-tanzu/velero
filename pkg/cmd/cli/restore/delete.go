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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/cli"
)

// NewDeleteCommand creates and returns a new cobra command for deleting restores.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := cli.NewDeleteOptions("restore")

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAMES]", use),
		Short: "Delete restores",
		Example: `  # Delete a restore named "restore-1".
  velero restore delete restore-1

  # Delete a restore named "restore-1" without prompting for confirmation.
  velero restore delete restore-1 --confirm

  # Delete restores named "restore-1" and "restore-2".
  velero restore delete restore-1 restore-2

  # Delete all restores labelled with "foo=bar".
  velero restore delete --selector foo=bar
	
  # Delete all restores.
  velero restore delete --all`,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, f, args))
			cmd.CheckError(Run(o))

		},
	}
	o.BindFlags(c.Flags())
	return c
}

// Run performs the deletion of restore(s).
func Run(o *cli.DeleteOptions) error {
	if !o.Confirm && !cli.GetConfirmation() {
		return nil
	}
	var (
		restores []*velerov1api.Restore
		errs     []error
	)

	switch {
	case len(o.Names) > 0:
		for _, name := range o.Names {
			restore, err := o.Client.VeleroV1().Restores(o.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, errors.WithStack(err))
				continue
			}
			restores = append(restores, restore)
		}
	default:
		selector := labels.Everything().String()
		if o.Selector.LabelSelector != nil {
			selector = o.Selector.String()
		}
		res, err := o.Client.VeleroV1().Restores(o.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			errs = append(errs, errors.WithStack(err))
		}

		for i := range res.Items {
			restores = append(restores, &res.Items[i])
		}
	}
	if len(restores) == 0 {
		fmt.Println("No restores found")
		return nil
	}
	for _, r := range restores {
		err := o.Client.VeleroV1().Restores(r.Namespace).Delete(context.TODO(), r.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, errors.WithStack(err))
			continue
		}
		fmt.Printf("Restore %q deleted\n", r.Name)
	}
	return kubeerrs.NewAggregate(errs)
}
