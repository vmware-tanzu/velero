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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cmd/util/flag"
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
	Names    []string
	Force    bool
	Confirm  bool
	All      bool
	Selector flag.LabelSelector

	client    clientset.Interface
	namespace string
}

func (o *DeleteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Force, "force", o.Force, "Forcefully delete the backup, potentially leaving orphaned cloud resources")
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, "Confirm forceful deletion")
	flags.BoolVar(&o.All, "all", o.All, "Delete all backups")
	flags.VarP(&o.Selector, "selector", "l", "Delete all backups matching this label selector")
}

func (o *DeleteOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	haveNames := len(args) > 0
	haveAll := o.All
	haveSelector := o.Selector.LabelSelector != nil

	if !haveNames && !haveAll && !haveSelector {
		return errors.New("you must specify backup name(s), --selector, or --all")
	}

	if haveAll && haveSelector {
		return errors.New("cannot set --all and --selector at the same time")
	}

	if haveNames && (haveAll || haveSelector) {
		return errors.New("cannot specify name(s) with --all or --selector")
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
	o.Names = args

	var err error
	o.client, err = f.Client()
	if err != nil {
		return nil
	}

	o.namespace = f.Namespace()

	return nil
}

func (o *DeleteOptions) Run() error {
	var backups []*v1.Backup
	var errs []error

	if len(o.Names) > 0 {
		for _, name := range o.Names {
			backup, err := o.client.ArkV1().Backups(o.namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, errors.WithStack(err))
				continue
			}

			backups = append(backups, backup)
		}
	}

	if o.All || o.Selector.LabelSelector != nil {
		// Default to all
		selector := labels.Everything().String()
		// Or use label selector
		if o.Selector.LabelSelector != nil {
			selector = o.Selector.String()
		}
		list, err := o.client.ArkV1().Backups(o.namespace).List(metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return errors.WithStack(err)
		}
		for i := range list.Items {
			backups = append(backups, &list.Items[i])
		}
	}

	if o.Force && !o.Confirm {
		// If the user didn't specify --confirm, we need to prompt for it
		if !getConfirmation() {
			return nil
		}
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return nil
	}

	for _, backup := range backups {
		if o.Force {
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
					errs = append(errs, errors.WithStack(err))
					continue
				}

				if _, err = o.client.ArkV1().Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes); err != nil {
					errs = append(errs, errors.WithStack(err))
					continue
				}
			}

			// Step 2: issue the delete ONLY if it has never been issued before
			if backup.DeletionTimestamp == nil {
				if err := o.client.ArkV1().Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
					errs = append(errs, errors.WithStack(err))
					continue
				}
			}

			fmt.Printf("Backup %q force-deleted.\n", backup.Name)
		} else {
			if err := o.client.ArkV1().Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
				errs = append(errs, errors.WithStack(err))
				continue
			}
			fmt.Printf("Requested deletion of backup %q. It will be removed once all associated data (disk snapshots, backup files, restores) are deleted.\n", backup.Name)
		}
	}

	return kubeerrors.NewAggregate(errs)
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
