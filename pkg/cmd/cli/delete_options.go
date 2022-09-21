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

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/vmware-tanzu/velero/pkg/client"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
)

// DeleteOptions contains parameters used for deleting a restore.
type DeleteOptions struct {
	*SelectOptions
	Confirm   bool
	Client    clientset.Interface
	Namespace string
}

func NewDeleteOptions(singularTypeName string) *DeleteOptions {
	o := &DeleteOptions{}
	o.SelectOptions = NewSelectOptions("delete", singularTypeName)
	return o
}

// Complete fills in the correct values for all the options.
func (o *DeleteOptions) Complete(f client.Factory, args []string) error {
	o.Namespace = f.Namespace()
	client, err := f.Client()
	if err != nil {
		return err
	}
	o.Client = client
	return o.SelectOptions.Complete(args)
}

// Validate validates the fields of the DeleteOptions struct.
func (o *DeleteOptions) Validate(c *cobra.Command, f client.Factory, args []string) error {
	if o.Client == nil {
		return errors.New("Velero client is not set; unable to proceed")
	}

	return o.SelectOptions.Validate()
}

// BindFlags binds options for this command to flags.
func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm deletion")
	o.SelectOptions.BindFlags(flags)
}

// GetConfirmation ensures that the user confirms the action before proceeding.
func GetConfirmation() bool {
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

// Xor returns true if exactly one of the provided values is true,
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
