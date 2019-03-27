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
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/dynamic"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/output"
	"github.com/heptio/velero/pkg/install"
	"github.com/heptio/velero/pkg/util/logging"
)

// InstallOptions collects all the options for installing Velero into a Kubernetes cluster.
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
	DryRun                    bool
}

// BindFlags adds command line values to the options struct.
func (o *InstallOptions) BindFlags(flags *pflag.FlagSet) {
	// TODO Send this string down into the deployment
	flags.StringVar(&o.DeploymentName, "deploy-name", o.DeploymentName, "name to apply to the Velero deployment")
	flags.StringVar(&o.BucketName, "bucket-name", o.BucketName, "bucket name in which to store backups")
	flags.StringVar(&o.Prefix, "prefix", o.Prefix, "prefix under the bucket in which to store backups")
	flags.StringVar(&o.BackupStorageProviderName, "backup-provider", o.BackupStorageProviderName, "provider name for backup storage")
	flags.StringVar(&o.Image, "image", o.Image, "image to use for the Velero server deployment")
	flags.BoolVar(&o.RestoreOnly, "restore-only", o.RestoreOnly, "run the server in restore-only mode")
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "don't create resources on the Kubernetes cluster")
}

// NewInstallOptions instantiates a new, default InstallOptions stuct.
func NewInstallOptions() *InstallOptions {
	return &InstallOptions{
		Namespace:      api.DefaultNamespace,
		DeploymentName: api.DefaultNamespace,
		Image:          install.DefaultImage,
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
	//TODO pass backup and volume config down
	resources, err := install.AllResources(o.Namespace, o.Image, o.BackupStorageProviderName, o.BucketName, o.Prefix)
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

	log.SetOutput(os.Stdout)
	// TODO: parse out log level
	logger := logging.DefaultLogger(logrus.DebugLevel)

	err = install.Install(client.NewDynamicFactory(dynamicClient), resources, logger)
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

	if o.BackupStorageProviderName == "" {
		return errors.Errorf("Backup storage proivder name must be provided")
	}

	return nil
}
