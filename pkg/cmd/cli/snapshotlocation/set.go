/*
Copyright The Velero contributors.

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

package snapshotlocation

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/output"
)

func NewSetCommand(f client.Factory, use string) *cobra.Command {
	o := NewSetOptions()

	c := &cobra.Command{
		Use:   use + " NAME",
		Short: "Set specific features for a snapshot location",
		Args:  cobra.ExactArgs(1),
		// Mark this command as hidden until more functionality is added
		// as part of https://github.com/vmware-tanzu/velero/issues/2426
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Run(c, f))
		},
	}

	o.BindFlags(c.Flags())
	return c
}

type SetOptions struct {
	Name string
}

func NewSetOptions() *SetOptions {
	return &SetOptions{}
}

func (o *SetOptions) BindFlags(*pflag.FlagSet) {
}

func (o *SetOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	return nil
}

func (o *SetOptions) Complete(args []string, f client.Factory) error {
	o.Name = args[0]
	return nil
}

func (o *SetOptions) Run(c *cobra.Command, f client.Factory) error {
	kbClient, err := f.KubebuilderClient()
	if err != nil {
		return err
	}

	location := &velerov1api.VolumeSnapshotLocation{}
	err = kbClient.Get(context.Background(), kbclient.ObjectKey{
		Namespace: f.Namespace(),
		Name:      o.Name,
	}, location)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := kbClient.Update(context.Background(), location, &kbclient.UpdateOptions{}); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Volume snapshot location %q configured successfully.\n", o.Name)
	return nil
}
