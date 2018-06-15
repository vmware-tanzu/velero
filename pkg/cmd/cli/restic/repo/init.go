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

package repo

import (
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	"github.com/heptio/ark/pkg/restic"
)

func NewInitCommand(f client.Factory) *cobra.Command {
	o := NewInitRepositoryOptions()

	c := &cobra.Command{
		Use:   "init NAMESPACE",
		Short: "initialize a restic repository for a specified namespace",
		Long:  "initialize a restic repository for a specified namespace",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(f, args))
			cmd.CheckError(o.Validate(f))
			cmd.CheckError(o.Run(f))
		},
	}

	o.BindFlags(c.Flags())

	return c
}

type InitRepositoryOptions struct {
	Namespace            string
	MaintenanceFrequency time.Duration
	*RepositoryKeyOptions

	kubeClient kclientset.Interface
	arkClient  clientset.Interface
}

func NewInitRepositoryOptions() *InitRepositoryOptions {
	return &InitRepositoryOptions{
		MaintenanceFrequency: restic.DefaultMaintenanceFrequency,
		RepositoryKeyOptions: NewRepositoryKeyOptions(),
	}
}

func (o *InitRepositoryOptions) BindFlags(flags *pflag.FlagSet) {
	flags.DurationVar(&o.MaintenanceFrequency, "maintenance-frequency", o.MaintenanceFrequency, "How often maintenance (i.e. restic prune & check) is run on the repository")

	o.RepositoryKeyOptions.BindFlags(flags)
}

func (o *InitRepositoryOptions) Complete(f client.Factory, args []string) error {
	o.Namespace = args[0]

	return o.RepositoryKeyOptions.Complete(f, args)
}

func (o *InitRepositoryOptions) Validate(f client.Factory) error {
	if o.MaintenanceFrequency <= 0 {
		return errors.Errorf("--maintenance-frequency must be greater than zero")
	}

	if err := o.RepositoryKeyOptions.Validate(f); err != nil {
		return err
	}

	kubeClient, err := f.KubeClient()
	if err != nil {
		return err
	}
	o.kubeClient = kubeClient

	if _, err := kubeClient.CoreV1().Namespaces().Get(o.Namespace, metav1.GetOptions{}); err != nil {
		return err
	}

	arkClient, err := f.Client()
	if err != nil {
		return err
	}
	o.arkClient = arkClient

	return nil
}

func (o *InitRepositoryOptions) Run(f client.Factory) error {
	if err := restic.NewRepositoryKey(o.kubeClient.CoreV1(), o.Namespace, o.keyBytes); err != nil {
		return err
	}

	repo := &v1.ResticRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Namespace(),
			Name:      o.Namespace,
		},
		Spec: v1.ResticRepositorySpec{
			MaintenanceFrequency: metav1.Duration{Duration: o.MaintenanceFrequency},
		},
	}

	_, err := o.arkClient.ArkV1().ResticRepositories(f.Namespace()).Create(repo)
	return errors.Wrap(err, "error creating ResticRepository")
}
