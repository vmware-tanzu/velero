/*
Copyright 2019 the Heptio Ark contributors.

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

package install

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"k8s.io/client-go/dynamic"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/flag"
	"github.com/heptio/velero/pkg/cmd/util/output"
	"github.com/heptio/velero/pkg/install"
)

// InstallOptions collects all the options for installing Velero into a Kubernetes cluster.
type InstallOptions struct {
	Namespace            string
	Image                string
	BucketName           string
	Prefix               string
	ProviderName         string
	RestoreOnly          bool
	Secret               string
	DryRun               bool
	BackupStorageConfig  flag.Map
	VolumeSnapshotConfig flag.Map
}

// BindFlags adds command line values to the options struct.
func (o *InstallOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BucketName, "bucket-name", o.BucketName, "name of the object storage bucket where backups should be stored")
	flags.StringVar(&o.Prefix, "prefix", o.Prefix, "prefix under which all Velero data should be stored within the bucket. Optional.")
	flags.StringVar(&o.ProviderName, "provider", o.ProviderName, "provider name for backup and volume storage")
	flags.StringVar(&o.Image, "image", o.Image, "image to use for the Velero and restic server deployment")
	flags.StringVar(&o.Secret, "secret", o.Secret, "file containing credentials to backup and volume provider")
	flags.Var(&o.BackupStorageConfig, "backup-location-config", "configuration to use for the backup storage location")
	flags.Var(&o.VolumeSnapshotConfig, "snapshot-location-config", "configuration to use for the volume snapshot location")
	flags.BoolVar(&o.RestoreOnly, "restore-only", o.RestoreOnly, "run the server in restore-only mode")
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "only print resources that would be installed, without sending them to the cluster")
}

// NewInstallOptions instantiates a new, default InstallOptions stuct.
func NewInstallOptions() *InstallOptions {
	return &InstallOptions{
		Namespace:            api.DefaultNamespace,
		Image:                install.DefaultImage,
		BackupStorageConfig:  flag.NewMap(),
		VolumeSnapshotConfig: flag.NewMap(),
	}
}

// NewCommand creates a cobra command.
func NewCommand(f client.Factory) *cobra.Command {
	o := NewInstallOptions()
	c := &cobra.Command{
		Use:   "install",
		Short: "Install Velero",
		Long:  "Install Velero into the Kubernetes cluster using provided information",
		Run: func(c *cobra.Command, args []string) {
			o.Namespace = c.Flag("namespace").Value.String()
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Run(c))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

// Run executes a command in the context of the provided arguments.
func (o *InstallOptions) Run(c *cobra.Command) error {
	realPath, err := filepath.Abs(o.Secret)
	if err != nil {
		return err
	}
	secretData, err := ioutil.ReadFile(realPath)
	if err != nil {
		return err
	}
	resources, err := install.AllResources(
		o.Namespace,
		o.Image,
		o.ProviderName,
		o.BucketName,
		o.Prefix,
		o.BackupStorageConfig.Data(),
		o.VolumeSnapshotConfig.Data(),
		secretData,
	)
	if err != nil {
		return err
	}

	if _, err := output.PrintWithFormat(c, resources); err != nil {
		return err
	}

	if o.DryRun {
		return nil
	}

	clientConfig, err := client.Config("", "", fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	err = install.Install(client.NewDynamicFactory(dynamicClient), resources, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

//Complete completes options for a command.
func (o *InstallOptions) Complete(args []string, f client.Factory) error {
	return nil
}

// Validate validates options provided to a command.
func (o *InstallOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	if o.BucketName == "" {
		return errors.Errorf("Bucket name must be provided")
	}

	if o.ProviderName == "" {
		return errors.Errorf("Backup storage proivder name must be provided")
	}

	if o.Secret == "" {
		return errors.Errorf("No secret file provided")
	}

	return nil
}
