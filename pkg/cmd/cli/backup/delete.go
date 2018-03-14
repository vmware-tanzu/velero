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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/heptio/ark/pkg/backup"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
)

func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := &DeleteOptions{}

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s NAME", use),
		Short: "Delete a backup",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Run())
		},
	}

	o.BindFlags(c.Flags())

	return c
}

type DeleteOptions struct {
	Name    string
	Confirm bool

	client    clientset.Interface
	namespace string
}

func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm deletion")
}

func (o *DeleteOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if o.client == nil {
		return errors.New("Ark client is not set; unable to proceed")
	}

	_, err := o.client.ArkV1().Backups(f.Namespace()).Get(args[0], metav1.GetOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (o *DeleteOptions) Complete(f client.Factory, args []string) error {
	o.Name = args[0]

	o.namespace = f.Namespace()

	client, err := f.Client()
	if err != nil {
		return err
	}
	o.client = client

	return nil
}

func (o *DeleteOptions) Run() error {
	if !o.Confirm && !getConfirmation() {
		// Don't do anything unless we get confirmation
		return nil
	}

	deleteRequest := backup.NewDeleteBackupRequest(o.Name)

	if _, err := o.client.ArkV1().DeleteBackupRequests(o.namespace).Create(deleteRequest); err != nil {
		return err
	}

	fmt.Printf("Request to delete backup %q submitted successfully.\nThe backup will be fully deleted after all associated data (disk snapshots, backup files, restores) are removed.\n", o.Name)
	return nil
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
