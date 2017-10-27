/*
Copyright 2017 Heptio Inc.

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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/flag"
	"github.com/heptio/ark/pkg/cmd/util/output"
)

func NewCreateCommand(f client.Factory, use string) *cobra.Command {
	o := NewCreateOptions()

	c := &cobra.Command{
		Use:   use + " NAME",
		Short: "Create a backup",
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Validate(c, args))
			cmd.CheckError(o.Complete(args))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

type CreateOptions struct {
	Name                    string
	TTL                     time.Duration
	SnapshotVolumes         flag.OptionalBool
	IncludeNamespaces       flag.StringArray
	ExcludeNamespaces       flag.StringArray
	IncludeResources        flag.StringArray
	ExcludeResources        flag.StringArray
	Labels                  flag.Map
	Selector                flag.LabelSelector
	IncludeClusterResources flag.OptionalBool
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		TTL:                     24 * time.Hour,
		IncludeNamespaces:       flag.NewStringArray("*"),
		Labels:                  flag.NewMap(),
		SnapshotVolumes:         flag.NewOptionalBool(nil),
		IncludeClusterResources: flag.NewOptionalBool(nil),
	}
}

func (o *CreateOptions) BindFlags(flags *pflag.FlagSet) {
	flags.DurationVar(&o.TTL, "ttl", o.TTL, "how long before the backup can be garbage collected")
	flags.Var(&o.IncludeNamespaces, "include-namespaces", "namespaces to include in the backup (use '*' for all namespaces)")
	flags.Var(&o.ExcludeNamespaces, "exclude-namespaces", "namespaces to exclude from the backup")
	flags.Var(&o.IncludeResources, "include-resources", "resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources)")
	flags.Var(&o.ExcludeResources, "exclude-resources", "resources to exclude from the backup, formatted as resource.group, such as storageclasses.storage.k8s.io")
	flags.Var(&o.Labels, "labels", "labels to apply to the backup")
	flags.VarP(&o.Selector, "selector", "l", "only back up resources matching this label selector")
	f := flags.VarPF(&o.SnapshotVolumes, "snapshot-volumes", "", "take snapshots of PersistentVolumes as part of the backup")
	// this allows the user to just specify "--snapshot-volumes" as shorthand for "--snapshot-volumes=true"
	// like a normal bool flag
	f.NoOptDefVal = "true"

	f = flags.VarPF(&o.IncludeClusterResources, "include-cluster-resources", "", "include cluster-scoped resources in the backup")
	f.NoOptDefVal = "true"
}

func (o *CreateOptions) Validate(c *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("you must specify only one argument, the backup's name")
	}

	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	return nil
}

func (o *CreateOptions) Complete(args []string) error {
	o.Name = args[0]
	return nil
}

func (o *CreateOptions) Run(c *cobra.Command, f client.Factory) error {
	arkClient, err := f.Client()
	if err != nil {
		return err
	}

	backup := &api.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: api.DefaultNamespace,
			Name:      o.Name,
			Labels:    o.Labels.Data(),
		},
		Spec: api.BackupSpec{
			IncludedNamespaces: o.IncludeNamespaces,
			ExcludedNamespaces: o.ExcludeNamespaces,
			IncludedResources:  o.IncludeResources,
			ExcludedResources:  o.ExcludeResources,
			LabelSelector:      o.Selector.LabelSelector,
			SnapshotVolumes:    o.SnapshotVolumes.Value,
			TTL:                metav1.Duration{Duration: o.TTL},
			IncludeClusterResources: o.IncludeClusterResources.Value,
		},
	}

	if printed, err := output.PrintWithFormat(c, backup); printed || err != nil {
		return err
	}

	_, err = arkClient.ArkV1().Backups(backup.Namespace).Create(backup)
	if err != nil {
		return err
	}

	fmt.Printf("Backup %q created successfully.\n", backup.Name)
	return nil
}
