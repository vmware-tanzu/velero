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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/controller"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/stringslice"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
)

func NewDeleteCommand(f client.Factory, use string) *cobra.Command {
	o := &DeleteOptions{}

	c := &cobra.Command{
		Use:   fmt.Sprintf("%s NAME", use),
		Short: "Delete a backup",
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Run())
		},
	}

	o.BindFlags(c.Flags())

	return c
}

type DeleteOptions struct {
	Name    string
	Force   bool
	Confirm bool

	client    clientset.Interface
	namespace string
}

func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Force, "force", o.Force, "Forcefully delete the backup, potentially leaving orphaned cloud resources")
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm forceful deletion")
}

func (o *DeleteOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if len(args) != 1 {
		return errors.New("you must specify only one argument, the backup's name")
	}

	kubeClient, err := f.KubeClient()
	if err != nil {
		return err
	}

	serverVersion, err := kubeutil.ServerVersion(kubeClient.Discovery())
	if err != nil {
		return err
	}

	if !serverVersion.AtLeast(controller.MinVersionForDelete) {
		return errors.Errorf("this command requires the Kubernetes server version to be at least %s", controller.MinVersionForDelete)
	}

	return nil
}

func (o *DeleteOptions) Complete(f client.Factory, args []string) error {
	o.Name = args[0]

	var err error
	o.client, err = f.Client()
	if err != nil {
		return nil
	}

	o.namespace = f.Namespace()

	return nil
}

func (o *DeleteOptions) Run() error {
	if o.Force {
		return o.forceDelete()
	}

	return o.normalDelete()
}

func (o *DeleteOptions) normalDelete() error {
	if err := o.client.ArkV1().Backups(o.namespace).Delete(o.Name, nil); err != nil {
		return err
	}

	fmt.Printf("Request to delete backup %q submitted successfully.\nThe backup will be fully deleted after all associated data (disk snapshots, backup files, restores) are removed.\n", o.Name)
	return nil
}

func (o *DeleteOptions) forceDelete() error {
	backup, err := o.client.ArkV1().Backups(o.namespace).Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	if !o.Confirm {
		// If the user didn't specify --confirm, we need to prompt for it
		if !getConfirmation() {
			return nil
		}
	}

	// Step 1: patch to remove our finalizer, if it's there
	if stringslice.Has(backup.Finalizers, v1.GCFinalizer) {
		patchMap := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers":      stringslice.Except(backup.Finalizers, v1.GCFinalizer),
				"resourceVersion": backup.ResourceVersion,
			},
		}

		patchBytes, err := json.Marshal(patchMap)
		if err != nil {
			return errors.WithStack(err)
		}

		if _, err = o.client.ArkV1().Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes); err != nil {
			return errors.WithStack(err)
		}
	}

	// Step 2: issue the delete ONLY if it has never been issued before
	if backup.DeletionTimestamp == nil {
		if err = o.client.ArkV1().Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
			return errors.WithStack(err)
		}
	}

	fmt.Printf("Backup %q force-deleted.\n", backup.Name)

	return nil
}

func getConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("WARNING: forcing deletion of a backup may result in resources in the cloud (disk snapshots, backup files) becoming orphaned.")
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
