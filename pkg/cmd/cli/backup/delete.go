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

package backup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/flag"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
)

// NewDeleteCommand creates a new command that deletes a backup.
func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := &DeleteOptions{}

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAMES]", use),
		Short: "Delete backups",
		Example: `  # delete a backup named "backup-1"
  ark backup delete backup-1

  # delete a backup named "backup-1" without prompting for confirmation
  ark backup delete backup-1 --confirm

  # delete backups named "backup-1" and "backup-2"
  ark backup delete backup-1 backup-2

  # delete all backups triggered by schedule "schedule-1"
  ark backup delete --selector ark-schedule=schedule-1
 
  # delete all backups
  ark backup delete --all
  `,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Run())
		},
	}

	o.BindFlags(c.Flags())

	return c
}

// DeleteOptions contains parameters for deleting a backup.
type DeleteOptions struct {
	Names    []string
	All      bool
	Selector flag.LabelSelector
	Confirm  bool

	client    clientset.Interface
	namespace string
}

// BindFlags binds options for this command to flags.
func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm deletion")
	flags.BoolVar(&o.All, "all", o.All, "Delete all backups")
	flags.VarP(&o.Selector, "selector", "l", "Delete all backups matching this label selector")
}

// Complete fills out the remainder of the parameters based on user input.
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

// Validate ensures all of the parameters have been filled in correctly.
func (o *DeleteOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if o.client == nil {
		return errors.New("Ark client is not set; unable to proceed")
	}

	var (
		hasNames    = len(o.Names) > 0
		hasAll      = o.All
		hasSelector = o.Selector.LabelSelector != nil
	)

	if !xor(hasNames, hasAll, hasSelector) {
		return errors.New("you must specify exactly one of: specific backup name(s), the --all flag, or the --selector flag")
	}

	return nil
}

// xor returns true if exactly one of the provided values is true,
// or false otherwise.
func xor(val bool, vals ...bool) bool {
	res := val

	for _, v := range vals {
		if res && v {
			return false
		}
		res = res || v
	}

	return res
}

// Run performs the delete backup operation.
func (o *DeleteOptions) Run() error {
	if !o.Confirm && !getConfirmation() {
		// Don't do anything unless we get confirmation
		return nil
	}

	var (
		backups []*arkv1api.Backup
		errs    []error
	)

	// get the list of backups to delete
	switch {
	case len(o.Names) > 0:
		for _, name := range o.Names {
			backup, err := o.client.ArkV1().Backups(o.namespace).Get(name, metav1.GetOptions{})
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

		res, err := o.client.ArkV1().Backups(o.namespace).List(metav1.ListOptions{LabelSelector: selector})
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

		if _, err := o.client.ArkV1().DeleteBackupRequests(o.namespace).Create(deleteRequest); err != nil {
			errs = append(errs, err)
			continue
		}

		fmt.Printf("Request to delete backup %q submitted successfully.\nThe backup will be fully deleted after all associated data (disk snapshots, backup files, restores) are removed.\n", b.Name)
	}

	return kubeerrs.NewAggregate(errs)
}

func getConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("Are you sure you want to continue (Y/N)? ")

		confirmation, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading user input: %v\n", err)
			return false
		}
		confirmation = strings.TrimSpace(confirmation)
		if len(confirmation) != 1 {
			continue
		}

		switch strings.ToLower(confirmation) {
		case "y":
			return true
		case "n":
			return false
		}
	}
}
