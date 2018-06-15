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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	clientset "github.com/heptio/ark/pkg/generated/clientset/versioned"
	"github.com/heptio/ark/pkg/restic"
)

func NewChangeKeyCommand(f client.Factory) *cobra.Command {
	o := NewChangeKeyOptions()

	c := &cobra.Command{
		Use:   "change-key REPOSITORY",
		Short: "change the key for a specified restic repository",
		Long:  "change the key for a specified restic repository",
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

type ChangeKeyOptions struct {
	Repository string
	*RepositoryKeyOptions

	kubeClient kclientset.Interface
	arkClient  clientset.Interface
}

func NewChangeKeyOptions() *ChangeKeyOptions {
	return &ChangeKeyOptions{
		RepositoryKeyOptions: NewRepositoryKeyOptions(),
	}
}

func (o *ChangeKeyOptions) BindFlags(flags *pflag.FlagSet) {
	o.RepositoryKeyOptions.BindFlags(flags)
}

func (o *ChangeKeyOptions) Complete(f client.Factory, args []string) error {
	o.Repository = args[0]

	return o.RepositoryKeyOptions.Complete(f, args)
}

func (o *ChangeKeyOptions) Validate(f client.Factory) error {
	if err := o.RepositoryKeyOptions.Validate(f); err != nil {
		return err
	}

	kubeClient, err := f.KubeClient()
	if err != nil {
		return err
	}
	o.kubeClient = kubeClient

	if _, err := kubeClient.CoreV1().Namespaces().Get(o.Repository, metav1.GetOptions{}); err != nil {
		return err
	}

	arkClient, err := f.Client()
	if err != nil {
		return err
	}
	o.arkClient = arkClient

	if _, err := arkClient.ArkV1().ResticRepositories(f.Namespace()).Get(o.Repository, metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}

func (o *ChangeKeyOptions) Run(f client.Factory) error {
	return restic.ChangeRepositoryKey(o.kubeClient.CoreV1(), o.Repository, o.keyBytes)
}
