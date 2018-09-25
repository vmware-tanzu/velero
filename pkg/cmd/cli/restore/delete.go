/*
Copyright 2017 the Heptio Ark contributors.

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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/cli"
	"github.com/heptio/ark/pkg/cmd/util/flag"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
)

// NewDeleteCommand creates and returns a new cobra command for deleting restores.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := &DeleteOptions{}
	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAMES]", use),
		Short: "Delete restores",
		Example: `	# delete a restore named "restore-1"
	ark restore delete restore-1

	# delete a restore named "restore-1" without prompting for confirmation
	ark restore delete restore-1 --confirm

	# delete restores named "restore-1" and "restore-2"
	ark restore delete restore-1 restore-2

	# delete all restores labelled with foo=bar"
	ark restore delete --selector foo=bar
	
	# delete all restores
	ark restore delete --all`,

		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, f, args))
			cmd.CheckError(o.Run())

		},
	}
	o.BindFlags(c.Flags())
	return c
}

// DeleteOptions contains parameters used for deleting a restore.
type DeleteOptions struct {
	Names     []string
	All       bool
	Selector  flag.LabelSelector
	Confirm   bool
	client    clientset.Interface
	namespace string
}

// Complete fills in the correct values for all the options.
func (o *DeleteOptions) Complete(f client.Factory, args []string) error {
	o.namespace = f.Namespace()
	client, err := f.Client()
	if err != nil {
		return err
	}
	o.client = client
	o.Names = args
	return nil
}

// Validate validates the fields of the DeleteOptions struct.
func (o *DeleteOptions) Validate(c *cobra.Command, f client.Factory, args []string) error {
	if o.client == nil {
		return errors.New("Ark client is not set; unable to proceed")
	}
	var (
		hasNames    = len(o.Names) > 0
		hasAll      = o.All
		hasSelector = o.Selector.LabelSelector != nil
	)
	if !cli.Xor(hasNames, hasAll, hasSelector) {
		return errors.New("you must specify exactly one of: specific restore name(s), the --all flag, or the --selector flag")
	}

	return nil
}

// Run performs the deletion of restore(s).
func (o *DeleteOptions) Run() error {
	if !o.Confirm && !cli.GetConfirmation() {
		return nil
	}
	var (
		restores []*arkv1api.Restore
		errs     []error
	)

	switch {
	case len(o.Names) > 0:
		for _, name := range o.Names {
			restore, err := o.client.ArkV1().Restores(o.namespace).Get(name, metav1.GetOptions{})
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
		res, err := o.client.ArkV1().Restores(o.namespace).List(metav1.ListOptions{
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
		err := o.client.ArkV1().Restores(r.Namespace).Delete(r.Name, nil)
		if err != nil {
			errs = append(errs, errors.WithStack(err))
			continue
		}
		fmt.Printf("Restore %q deleted\n", r.Name)
	}
	return kubeerrs.NewAggregate(errs)
}

// BindFlags binds the options for this command to the flags.
func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm deletion")
	flags.BoolVar(&o.All, "all", o.All, "Delete all restores")
	flags.VarP(&o.Selector, "selector", "l", "Delete all restores matching this label selector")
}
