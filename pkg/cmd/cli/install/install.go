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
	"time"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/output"
	"github.com/heptio/velero/pkg/install"
)

const defaultImage = "gcr.io/heptio/velero:latest"

type InstallOptions struct {
	Namespace                 string
	DeploymentName            string
	Image                     string
	BucketName                string
	Prefix                    string
	BackupStorageProviderName string
	RestoreOnly               bool
	LogLevel                  string
	ResticTimeout             time.Duration
	SecretName                string
}

// Args:
//   secret
//   plugin-dir
//   basically all the server args?
// Flags
//   dry-run (yaml dump)

func (o *InstallOptions) BindFlags(flags *pflag.FlagSet) {
	// TODO Send this string down into the deployment
	flags.StringVar(&o.DeploymentName, "deploy-name", "", "name to apply to the Velero deployment")
	flags.StringVar(&o.BucketName, "bucket-name", "", "bucket name in which to store backups")
	flags.StringVar(&o.Prefix, "prefix", "", "prefix under the bucket in which to store backups")
	flags.StringVar(&o.BackupStorageProviderName, "backup-provider", "", "provider name for backup storage")
	flags.StringVar(&o.Image, "image", "", "image to use for the Velero server deployment")
	flags.BoolVar(&o.RestoreOnly, "restore-only", false, "run the server in restore-only mode")
}

func NewInstallOptions() *InstallOptions {
	return &InstallOptions{}
}

func NewCommand(f client.Factory) *cobra.Command {
	o := NewInstallOptions()
	c := &cobra.Command{
		Use:   "install",
		Short: "Install Velero",
		Long:  "Install Velero",
		Run: func(c *cobra.Command, args []string) {
			o.Namespace = c.Flag("namespace").Value.String()
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Run(c))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

func (o *InstallOptions) Run(c *cobra.Command) error {

	resources, err := install.AllResources(o.Namespace, o.Image, o.BackupStorageProviderName, o.BucketName, o.Prefix)
	if err != nil {
		return err
	}

	// TODO: Wrap the resources in a list instead o just printing with separators
	if _, err := output.PrintWithFormat(c, resources); err != nil {
		return err
	}

	return nil
}

func (o *InstallOptions) Complete(args []string, f client.Factory) error {
	if o.Namespace == "" {
		o.Namespace = api.DefaultNamespace
	}
	if o.DeploymentName == "" {
		o.DeploymentName = api.DefaultNamespace
	}
	if o.Image == "" {
		o.Image = defaultImage
	}
	return nil
}
