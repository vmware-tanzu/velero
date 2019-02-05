/*
Copyright 2018 the Heptio Ark contributors.

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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/flag"
	"github.com/heptio/velero/pkg/cmd/util/output"
)

func NewCreateCommand(f client.Factory, use string) *cobra.Command {
	o := NewCreateOptions()

	c := &cobra.Command{
		Use:   use + " NAME",
		Short: "Create a backup storage location",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

type CreateOptions struct {
	Name     string
	Provider string
	Bucket   string
	Prefix   string
	Config   flag.Map
	Labels   flag.Map
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		Config: flag.NewMap(),
	}
}

func (o *CreateOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.Provider, "provider", o.Provider, "name of the backup storage provider (e.g. aws, azure, gcp)")
	flags.StringVar(&o.Bucket, "bucket", o.Bucket, "name of the object storage bucket where backups should be stored")
	flags.StringVar(&o.Prefix, "prefix", o.Prefix, "prefix under which all Velero data should be stored within the bucket. Optional.")
	flags.Var(&o.Config, "config", "configuration key-value pairs")
	flags.Var(&o.Labels, "labels", "labels to apply to the backup storage location")
}

func (o *CreateOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	if o.Provider == "" {
		return errors.New("--provider is required")
	}

	if o.Bucket == "" {
		return errors.New("--bucket is required")
	}

	return nil
}

func (o *CreateOptions) Complete(args []string, f client.Factory) error {
	o.Name = args[0]
	return nil
}

func (o *CreateOptions) Run(c *cobra.Command, f client.Factory) error {
	backupStorageLocation := &api.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Namespace(),
			Name:      o.Name,
			Labels:    o.Labels.Data(),
		},
		Spec: api.BackupStorageLocationSpec{
			Provider: o.Provider,
			StorageType: api.StorageType{
				ObjectStorage: &api.ObjectStorageLocation{
					Bucket: o.Bucket,
					Prefix: o.Prefix,
				},
			},
			Config: o.Config.Data(),
		},
	}

	if printed, err := output.PrintWithFormat(c, backupStorageLocation); printed || err != nil {
		return err
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	if _, err := client.VeleroV1().BackupStorageLocations(backupStorageLocation.Namespace).Create(backupStorageLocation); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Backup storage location %q configured successfully.\n", backupStorageLocation.Name)
	return nil
}
